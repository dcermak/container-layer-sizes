package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"

	internal "github.com/dcermak/container-layer-sizes/pkg"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/unshare"

	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	archiver "github.com/mholt/archiver/v4"
	"github.com/syndtr/gocapability/capability"

	logrus "github.com/sirupsen/logrus"
)

// capabilities for running in a user namespace
var neededCapabilities = []capability.Cap{
	capability.CAP_CHOWN,
	capability.CAP_DAC_OVERRIDE,
	capability.CAP_FOWNER,
	capability.CAP_FSETID,
	capability.CAP_MKNOD,
	capability.CAP_SETFCAP,
}

const (
	addr = ":5050"
)

var log = logrus.New()

var backgroundContext = context.Background()

type TaskState uint

const (
	TaskStateNew = iota
	TaskStatePulling
	TaskStateExtracting
	TaskStateAnalyzing
	TaskStateFinished
	TaskStateError
)

func TaskStateToStr(s TaskState) string {
	switch s {
	case TaskStateNew:
		return "Task is new"
	case TaskStatePulling:
		return "Pulling image"
	case TaskStateExtracting:
		return "Extracting image"
	case TaskStateAnalyzing:
		return "Analyzing image"
	case TaskStateFinished:
		return "Task is finished"
	case TaskStateError:
		return "Task failed"
	default:
		panic(fmt.Sprintf("Invalid TaskState %d", s))
	}
}

type LayerDownloadProgress struct {
	TotalSize  int64  `json:"total_size"`
	Downloaded uint64 `json:"downloaded"`
}

type Task struct {
	/// URL of the container image
	Image string `json:"image"`

	/// Current state of this task, can be converted to a string via `TaskStateToStr`
	State TaskState `json:"state"`

	/// Metadata of the container image
	Metadata string `json:"metadata"`

	/// the current progress for downloading the image as it would have been
	/// created by `podman pull`
	PullProgress map[string]LayerDownloadProgress `json:"pull_progress"`

	///
	ImageInfo *types.ImageInspectInfo `json:"image_info"`

	/// an error if any occurred
	error error

	layers *internal.LayerSizes

	tempdir string

	// the reference to the "remote" image (usually this is expected to
	// exist on a registry, but it can actually be a local one as well ;-))
	remoteReference types.ImageReference

	// reference to the image in the local containers storage
	localReference types.ImageReference

	ctx    context.Context
	cancel context.CancelFunc
}

func (t *Task) MarshalJSON() ([]byte, error) {
	type Alias Task

	var errMsg string
	if t.error == nil {
		errMsg = ""
	} else {
		errMsg = t.error.Error()
	}
	return json.Marshal(&struct {
		Error string `json:"error"`
		*Alias
	}{Error: errMsg, Alias: (*Alias)(t)})
}

func NewTask(imageUrl string) (*Task, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	layers := make(internal.LayerSizes)

	var remoteReference, localReference types.ImageReference
	// the image url does not specify a transport => assume it's a url to a registry
	if parts := strings.Split(imageUrl, ":"); len(parts) < 2 {
		ref, err := reference.ParseNormalizedNamed(imageUrl)
		if err != nil {
			return nil, err
		}

		remoteReference, err = docker.NewReference(reference.TagNameOnly(ref))
		if err != nil {
			return nil, err
		}

		localReference, err = storage.Transport.ParseReference(imageUrl)
		if err != nil {
			return nil, err
		}
	} else {
		// the image includes the transport so we just parse it from all transports
		remoteReference, err = alltransports.ParseImageName(imageUrl)
		if err != nil {
			return nil, err
		}

		// but for the local storage we have to omit the transport part,
		// as ParseReference must not be called on an url with the
		// transport
		urlWithoutTransport := strings.Join(parts[1:], ":")
		localReference, err = storage.Transport.ParseReference(urlWithoutTransport)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(backgroundContext, 5*time.Minute)
	return &Task{
			Image:           imageUrl,
			State:           TaskStateNew,
			layers:          &layers,
			tempdir:         tempdir,
			error:           nil,
			ctx:             ctx,
			cancel:          cancel,
			remoteReference: remoteReference,
			localReference:  localReference,
		},
		nil
}

func (t *Task) Process() {
	var err error

	setError := func(e error) {
		log.WithFields(
			logrus.Fields{"error": e, "task": t},
		).Error("Error occurred when processing the task")

		t.error = e
		t.State = TaskStateError
	}

	t.State = TaskStatePulling

	t.ImageInfo, err = InspectRemoteImage(t.remoteReference, t.ctx)
	if err != nil {
		setError(err)
		return
	}

	t.PullProgress = make(map[string]LayerDownloadProgress, len(t.ImageInfo.Layers))
	for _, layerDigest := range t.ImageInfo.Layers {
		t.PullProgress[layerDigest] = LayerDownloadProgress{TotalSize: int64(-1), Downloaded: 0}
	}

	opts := copy.Options{
		ProgressInterval: time.Second,
		Progress:         make(chan types.ProgressProperties),
	}

	go func() {
		for p := range opts.Progress {
			curProgress, ok := t.PullProgress[string(p.Artifact.Digest)]
			f := logrus.Fields{"task": t, "progress_report": p}
			if !ok {
				log.WithFields(f).Error("Received progress report for an unknown layer")
			} else {
				if curProgress.Downloaded > p.Offset {
					log.WithFields(
						f,
					).Error("Downloaded size is smaller then previous value")
				}
				if curProgress.TotalSize != 0 && curProgress.TotalSize != p.Artifact.Size {
					log.WithFields(f).Error("Total size of the layer changed")
				}
			}

			downloaded := p.Offset
			if p.Event == types.ProgressEventDone || p.Event == types.ProgressEventSkipped {
				downloaded = uint64(p.Artifact.Size)
			}

			t.PullProgress[string(p.Artifact.Digest)] = LayerDownloadProgress{
				TotalSize:  p.Artifact.Size,
				Downloaded: downloaded,
			}
		}
	}()

	if t.remoteReference.Transport().Name() == t.localReference.Transport().Name() {
		log.WithFields(
			logrus.Fields{
				"remote reference": t.remoteReference.StringWithinTransport(),
				"local reference":  t.localReference.StringWithinTransport(),
			},
		).Trace("Not pulling image into local storage, as it is already present locally")
	} else if _, err := CopyImage(t.remoteReference, t.localReference, &t.ctx, &opts); err != nil {
		if ctxErr := t.ctx.Err(); ctxErr == context.Canceled {
			log.WithFields(
				logrus.Fields{"error": err, "context_error": ctxErr, "task": t},
			).Error("Task has been canceled")
			return
		} else if ctxErr == context.DeadlineExceeded {
			log.WithFields(
				logrus.Fields{"error": err, "context_error": ctxErr, "task": t},
			).Error("Task has exceeded the deadline")
			return
		} else {
			if ctxErr != nil {
				setError(ctxErr)
			} else {
				setError(err)
			}
			return
		}
	}

	t.State = TaskStateExtracting
	m, err := CopyImage(t.localReference, t.ociLocalReference, &t.ctx, nil)
	if err != nil {
		setError(err)
		return
	}

	var manifest Manifest
	err = json.Unmarshal(m, &manifest)
	if err != nil {
		setError(err)
		return
	}

	m, err = ReadOciImageMetadata(t.tempdir, manifest)
	if err != nil {
		setError(err)
		return
	}
	t.Metadata = string(m)

	t.State = TaskStateAnalyzing
	layers, err := CalculateContainerLayerSizes(t.tempdir, manifest)
	if err != nil {
		setError(err)
		return
	}

	t.layers = &layers
	t.State = TaskStateFinished
}

func (t *Task) Cleanup() error {
	t.cancel()
	return os.RemoveAll(t.tempdir)
}

type TaskQueue struct {
	tasks map[string]*Task
}

func NewTaskQueue() TaskQueue {
	return TaskQueue{tasks: make(map[string]*Task)}
}

func (tq *TaskQueue) CleanupQueue() []error {
	errors := make([]error, 1)
	for _, t := range tq.tasks {
		if err := t.Cleanup(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (tq *TaskQueue) AddTask(imageUrl string) (string, *Task, error) {
	id := fmt.Sprint(uuid.New())

	if t, err := NewTask(imageUrl); err != nil {
		return "", nil, err
	} else {
		tq.tasks[id] = t
		return id, t, nil
	}
}

func (tq *TaskQueue) GetTask(id string) (*Task, error) {
	if t, ok := tq.tasks[id]; !ok {
		return nil, errors.New(fmt.Sprintf("Non existing task id %s", id))
	} else {
		return t, nil
	}
}

func (tq *TaskQueue) RemoveTask(id string) error {
	if t, ok := tq.tasks[id]; !ok {
		return errors.New(fmt.Sprintf("Non existing task id %s", id))
	} else {
		err := t.Cleanup()
		delete(tq.tasks, id)
		return err
	}
}

type ExtractedDigest struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        ExtractedDigest   `json:"config"`
	Layers        []ExtractedDigest `json:"layers"`
}

func InspectRemoteImage(ref types.ImageReference, ctx context.Context) (info *types.ImageInspectInfo, err error) {
	err = nil
	info = nil
	sys := types.SystemContext{}

	log.WithFields(
		logrus.Fields{"reference": ref.StringWithinTransport()},
	).Info("Inspecting image")

	imgSrc, err := ref.NewImageSource(ctx, &sys)
	if err != nil {
		return
	}
	defer imgSrc.Close()

	img, err := image.FromUnparsedImage(ctx, nil, image.UnparsedInstance(imgSrc, nil))
	if err != nil {
		log.Trace("Failed to generate a new image from an unparsed image")
		return
	}
	return img.Inspect(ctx)
}

/// Copies an image from the source reference to the destination indicated by destRef.
///
/// If `ctx` is non-nil, then it is used for the actual copy process.
/// If it is nil, then the backgroundContext is used instead.
///
/// `opts` are forwarded to the call of `copy.Image`
func CopyImage(sourceRef types.ImageReference, destRef types.ImageReference, ctx *context.Context, opts *copy.Options) ([]byte, error) {
	log.WithFields(
		logrus.Fields{
			"source reference":      sourceRef.StringWithinTransport(),
			"destination reference": destRef.StringWithinTransport(),
		},
	).Info("Copying an image")

	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return nil, err
	}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	defer policyCtx.Destroy()

	var c context.Context
	if ctx != nil {
		c = *ctx
	} else {
		c = backgroundContext
	}
	if manifest, err := copy.Image(c, policyCtx, destRef, sourceRef, opts); err != nil {
		return nil, err
	} else {
		return manifest, nil
	}
}

func ReadOciImageMetadata(unpackedImageDest string, manifest Manifest) ([]byte, error) {
	mediatype := manifest.Config.MediaType
	if mediatype != "application/vnd.oci.image.config.v1+json" {
		return nil, errors.New(fmt.Sprintf("Invalid media type: %s", mediatype))
	}

	digest := strings.Split(manifest.Config.Digest, ":")
	if len(digest) != 2 {
		return nil, errors.New(fmt.Sprintf("invalid digest: %s", digest))
	}

	return ioutil.ReadFile(filepath.Join(unpackedImageDest, "blobs", "sha256", digest[1]))
}

func CalculateContainerLayerSizes(unpackedImageDest string, manifest Manifest) (internal.LayerSizes, error) {
	layers := make(internal.LayerSizes)

	for _, layer := range manifest.Layers {
		mediatype := layer.MediaType
		if mediatype != "application/vnd.oci.image.layer.v1.tar+gzip" {
			return nil, errors.New(fmt.Sprintf("Invalid media type: %s", mediatype))
		}

		digest := strings.Split(layer.Digest, ":")
		if len(digest) != 2 {
			return nil, errors.New(fmt.Sprintf("invalid digest: %s", digest))
		}

		root := internal.MakeDir("/")

		archivePath := filepath.Join(unpackedImageDest, "blobs", "sha256", digest[1])
		f, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}

		format, err := archiver.Identify(archivePath, f)
		if err != nil {
			return nil, err
		}

		if ex, ok := format.(archiver.Extractor); ok {
			err := ex.Extract(backgroundContext, f, nil, func(ctx context.Context, f archiver.File) error {
				if f.IsDir() {
					return nil
				}
				root.InsertIntoDir(f.NameInArchive, f.Size())
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New(fmt.Sprintf("%u is not an Extractor", ex))
		}

		layers[digest[1]] = root
	}

	return layers, nil
}

func main() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.TraceLevel)

	rootless := true
	if len(os.Args[1:]) == 1 && os.Args[1] == "--no-rootless" {
		rootless = false
	}

	if rootless {
		reexec.Init()

		capabilities, err := capability.NewPid(0)
		if err != nil {
			panic(err)
		}
		for _, cap := range neededCapabilities {
			if !capabilities.Get(capability.EFFECTIVE, cap) {
				// We miss a capability we need, create a user namespaces
				unshare.MaybeReexecUsingUserNamespace(true)
			}
		}
	}

	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)

	tq := NewTaskQueue()
	defer tq.CleanupQueue()

	jobs := make(chan *Task)
	go func() {
		for t := range jobs {
			t.Process()
		}
	}()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Error parsing form data: %s", err), 400)
			return
		}
		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "Parameter id was not provided", 400)
			return
		}

		t, err := tq.GetTask(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Got an error fetching the task with the id %s: %s", id, err.Error()), 500)
			return
		}

		if t.State != TaskStateFinished {
			http.Error(
				w,
				fmt.Sprintf(
					"Cannot get data from task %s, task is not in finished state (got state %s)",
					id, TaskStateToStr(t.State)),
				500,
			)
			return
		}

		if j, err := json.Marshal(t.layers); err != nil {
			log.WithFields(logrus.Fields{
				"layers": t.layers,
				"id":     id,
				"state":  t.State,
				"error":  err,
			}).Error("Failed to marshal the layers to json")

			http.Error(w, err.Error(), 500)
		} else {
			fmt.Fprint(w, string(j))
		}
		log.WithFields(logrus.Fields{"id": id}).Trace("send data, removing task from queue")
		tq.RemoveTask(id)
	})
	http.HandleFunc("/task", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Error parsing form data: %s", err), 400)
			return
		}

		switch r.Method {
		case "POST":
			img := r.PostFormValue("image")
			if img == "" {
				http.Error(w, "No image provided", 400)
				return
			}

			if id, t, err := tq.AddTask(img); err != nil {
				http.Error(w, fmt.Sprintf("Error creating task: %s", err), 400)
			} else {
				jobs <- t
				fmt.Fprintf(w, id)
			}
			return
		case "GET":
			fallthrough
		case "DELETE":
			id := r.FormValue("id")
			if id == "" {
				http.Error(w, "No task id provided", 400)
				return
			}
			if task, err := tq.GetTask(id); err != nil {
				http.Error(w, err.Error(), 400)
			} else if r.Method == "GET" {
				if j, err := json.Marshal(task); err != nil {
					http.Error(w, err.Error(), 500)
				} else {
					fmt.Fprint(w, string(j))
				}
			} else if r.Method == "DELETE" {
				log.WithFields(
					logrus.Fields{"id": id},
				).Debug("Removing task on user request")

				if err := tq.RemoveTask(id); err != nil {
					log.WithFields(
						logrus.Fields{"error": err},
					).Error("Failed to remove task")

					http.Error(w, err.Error(), 500)
					return
				}

				log.WithFields(
					logrus.Fields{"id": id},
				).Debug("Canceled and removed task")
			}
			return
		}
	})
	fmt.Printf("Ready. Listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
