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
	dockerArchiveTransport "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/unshare"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"

	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	archiver "github.com/mholt/archiver/v4"
	logrus "github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
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

func reexecForRootlessStorage() error {
	reexec.Init()

	capabilities, err := capability.NewPid(0)
	if err != nil {
		return err
	}
	for _, cap := range neededCapabilities {
		if !capabilities.Get(capability.EFFECTIVE, cap) {
			// We miss a capability we need, create a user namespaces
			unshare.MaybeReexecUsingUserNamespace(true)
		}
	}
	return nil
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

type ContainerImage struct {
	/// URL or otherwise fully qualified name of this container image,
	/// excluding the tag
	Image string

	/// The tag via which this image was fetched
	Tag string

	Transport string

	Manifest Manifest

	///
	ImageInfo *types.ImageInspectInfo

	/// The digest of this image in oci form
	ImageDigest string

	layers *internal.LayerSizes

	// the reference to the "remote" image (usually this is expected to
	// exist on a registry, but it can actually be a local one as well ;-))
	remoteReference types.ImageReference

	// reference to the image in the local containers storage
	localReference types.ImageReference

	ociLocalReference types.ImageReference
}

type Task struct {
	/// The image belonging to this task
	Image ContainerImage

	/// Current state of this task, can be converted to a string via `TaskStateToStr`
	State TaskState `json:"state"`

	/// the current progress for downloading the image as it would have been
	/// created by `podman pull`
	PullProgress map[string]LayerDownloadProgress `json:"pull_progress"`

	/// an error if any occurred
	error error

	tempdir string

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

func getNameTagFromUrl(u string) (name string, tag string, err error) {
	nameAndTag := strings.Split(u, ":")

	if len(nameAndTag) == 0 || len(nameAndTag) > 2 {
		return "", "", errors.New(
			fmt.Sprintf("Invalid image name: %s", u),
		)
	} else if len(nameAndTag) == 2 {
		return nameAndTag[0], nameAndTag[1], nil
	} else {
		return u, "", nil
	}
}

func NewTask(imageUrl string) (*Task, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	layers := make(internal.LayerSizes)

	var remoteReference, localReference types.ImageReference

	parts := strings.Split(imageUrl, ":")

	transportName := ""
	tag := ""

	// drop the transport from the url
	urlWithoutTransport := strings.Join(parts[1:], ":")

	remoteReference, err = alltransports.ParseImageName(imageUrl)
	if err != nil {
		return nil, err
	}
	if transport := remoteReference.Transport(); transport == nil {
		return nil, errors.New(fmt.Sprintf("Image %s contains no valid transport", imageUrl))
	} else {
		transportName = transport.Name()
	}

	if transportName == "docker" {
		// docker transport urls contain // after `docker:`, drop that one as well
		urlWithoutTransport = urlWithoutTransport[2:]

		ref, err := reference.ParseNormalizedNamed(urlWithoutTransport)
		if err != nil {
			return nil, err
		}

		remoteReference, err = docker.NewReference(reference.TagNameOnly(ref))
		if err != nil {
			return nil, err
		}

		localReference, err = storage.Transport.ParseReference(urlWithoutTransport)
		if err != nil {
			return nil, err
		}

		urlWithoutTransport, tag, err = getNameTagFromUrl(urlWithoutTransport)
		if err != nil {
			return nil, err
		}
	} else {
		imageName := ""

		if transportName == "docker-archive" {
			sys := types.SystemContext{}
			archivePath := strings.Join(parts[1:], ":")
			r, err := dockerArchiveTransport.NewReader(&sys, archivePath)
			if err != nil {
				return nil, err
			}
			refs, err := r.List()
			if err != nil {
				return nil, err
			}
			if len(refs) > 0 {
				// the stringwithinTransport is for some reason prepended with the archive's path plus a colon…
				// we remove it, if it is there
				imageName = refs[0][0].StringWithinTransport()
				imageName = strings.Replace(imageName, archivePath+":", "", 1)
			}
		} else if transportName == "containers-storage" {
			localRef, err := alltransports.ParseImageName(imageUrl)
			if err != nil {
				return nil, err
			}
			if dockerRef := localRef.DockerReference(); dockerRef != nil {
				imageName = dockerRef.Name()
			}
		}
		if imageName == "" {
			return nil, errors.New(
				fmt.Sprintf(
					"Could not infer the name of the image with the url %s",
					imageUrl,
				),
			)
		}

		// we now need to do some fiddling with the tags:
		// imageName might not include the tag, but imageUrl might
		// => if imageName has a tag => use it
		// => if imageName has no tag => take the one from imageUrl
		var name string
		name, tag, err = getNameTagFromUrl(imageName)
		if err != nil {
			return nil, err
		}

		if tag == "" {
			_, tag, err = getNameTagFromUrl(urlWithoutTransport)
			if err != nil {
				return nil, err
			}
		}

		urlWithoutTransport = name
	}

	localReference, err = storage.Transport.ParseReference(urlWithoutTransport)
	if err != nil {
		return nil, err
	}

	if tag == "" {
		tag = "latest"
	}

	ociLocalReference, err := layout.Transport.ParseReference(tempdir + ":" + tag)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(backgroundContext, 5*time.Minute)

	Image := ContainerImage{
		Image:             urlWithoutTransport,
		Tag:               tag,
		Transport:         transportName,
		layers:            &layers,
		remoteReference:   remoteReference,
		localReference:    localReference,
		ociLocalReference: ociLocalReference,
	}

	return &Task{
			Image:   Image,
			State:   TaskStateNew,
			tempdir: tempdir,
			error:   nil,
			ctx:     ctx,
			cancel:  cancel,
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

	if t.Image.ImageInfo == nil {
		t.Image.ImageInfo, err = InspectImage(t.Image.remoteReference, t.ctx)
		if err != nil {
			setError(err)
			return
		}
	}

	t.PullProgress = make(map[string]LayerDownloadProgress, len(t.Image.ImageInfo.Layers))
	for _, layerDigest := range t.Image.ImageInfo.Layers {
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

	if t.Image.remoteReference.Transport().Name() == t.Image.localReference.Transport().Name() {
		log.WithFields(
			logrus.Fields{
				"remote reference": t.Image.remoteReference.StringWithinTransport(),
				"local reference":  t.Image.localReference.StringWithinTransport(),
			},
		).Trace("Not pulling image into local storage, as it is already present locally")
	} else if _, err := CopyImage(t.Image.remoteReference, t.Image.localReference, &t.ctx, &opts); err != nil {
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
	m, err := CopyImage(
		t.Image.localReference,
		t.Image.ociLocalReference,
		&t.ctx,
		&copy.Options{RemoveSignatures: true},
	)
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
	t.Image.ImageDigest = manifest.Config.Digest

	t.State = TaskStateAnalyzing
	layers, err := CalculateContainerLayerSizes(t.tempdir, manifest)
	if err != nil {
		setError(err)
		return
	}

	history, err := ReadHistoryFromOciArchive(t.tempdir, t.Image.Tag)
	if err != nil {
		setError(err)
		return
	}
	for digest, createdBy := range history {
		d := strings.Split(digest, ":")
		if len(d) != 2 || d[0] != "sha256" {
			log.WithFields(
				logrus.Fields{"digest": digest},
			).Error("Umoci found an invalid digest")
			continue
		}
		if layer, ok := layers[d[1]]; ok {
			layer.CreatedBy = createdBy
			layers[d[1]] = layer
		} else {
			log.WithFields(
				logrus.Fields{"digest": digest},
			).Error("Umoci found a digest that is not present in the extracted layers")
		}
	}

	t.Image.layers = &layers
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

func InspectImage(ref types.ImageReference, ctx context.Context) (*types.ImageInspectInfo, error) {
	sys := types.SystemContext{}

	log.WithFields(
		logrus.Fields{"reference": ref.StringWithinTransport()},
	).Info("Inspecting image")

	imgSrc, err := ref.NewImageSource(ctx, &sys)
	if err != nil {
		return nil, err
	}
	defer imgSrc.Close()

	img, err := image.FromUnparsedImage(ctx, nil, image.UnparsedInstance(imgSrc, nil))
	if err != nil {
		log.Trace("Failed to generate a new image from an unparsed image")
		return nil, err
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

		root := internal.NewLayer()

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

func ReadHistoryFromOciArchive(imagePath string, tagName string) (map[string]string, error) {
	// this is mostly stolen from the umoci stat command
	engine, err := dir.Open(imagePath)
	if err != nil {
		return nil, err
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	manifestDescriptorPaths, err := engineExt.ResolveReference(backgroundContext, tagName)
	if err != nil {
		return nil, err
	}
	if len(manifestDescriptorPaths) == 0 {
		return nil, errors.New(fmt.Sprintf("tag not found: %s", tagName))
	}
	if len(manifestDescriptorPaths) != 1 {
		return nil, errors.New(fmt.Sprintf("tag is ambiguous: %s", tagName))
	}
	manifestDescriptor := manifestDescriptorPaths[0].Descriptor()

	if manifestDescriptor.MediaType != ispec.MediaTypeImageManifest {
		return nil, errors.New(fmt.Sprintf("descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestDescriptor.MediaType))
	}

	ms, err := umoci.Stat(backgroundContext, engineExt, manifestDescriptor)
	if err != nil {
		return nil, err
	}

	res := make(map[string]string, len(ms.History))
	for _, histEntry := range ms.History {
		if histEntry.Layer != nil {
			res[string(histEntry.Layer.Digest)] = histEntry.CreatedBy
		}
	}
	return res, nil
}

func main() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.TraceLevel)

	rootless := true
	if len(os.Args[1:]) == 1 && os.Args[1] == "--no-rootless" {
		rootless = false
	}

	if rootless {
		err := reexecForRootlessStorage()
		if err != nil {
			panic(err)
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
			http.Error(w, fmt.Sprintf("Error parsing form data: %s", err), http.StatusBadRequest)
			return
		}
		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "Parameter id was not provided", http.StatusBadRequest)
			return
		}

		t, err := tq.GetTask(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Got an error fetching the task with the id %s: %s", id, err.Error()), http.StatusInternalServerError)
			return
		}

		if t.State != TaskStateFinished {
			http.Error(
				w,
				fmt.Sprintf(
					"Cannot get data from task %s, task is not in finished state (got state %s)",
					id, TaskStateToStr(t.State)),
				http.StatusInternalServerError,
			)
			return
		}

		if j, err := json.Marshal(t.Image.layers); err != nil {
			log.WithFields(logrus.Fields{
				"layers": t.Image.layers,
				"id":     id,
				"state":  t.State,
				"error":  err,
			}).Error("Failed to marshal the layers to json")

			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			fmt.Fprint(w, string(j))
		}
		log.WithFields(logrus.Fields{"id": id}).Trace("send data, removing task from queue")
		tq.RemoveTask(id)
	})

	http.HandleFunc("/task", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Error parsing form data: %s", err), http.StatusBadRequest)
			return
		}

		switch r.Method {
		case "POST":
			img := r.PostFormValue("image")
			if img == "" {
				http.Error(w, "No image provided", http.StatusBadRequest)
				return
			}

			if id, t, err := tq.AddTask(img); err != nil {
				http.Error(w, fmt.Sprintf("Error creating task: %s", err), http.StatusBadRequest)
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
				http.Error(w, "No task id provided", http.StatusBadRequest)
				return
			}
			if task, err := tq.GetTask(id); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else if r.Method == "GET" {
				if j, err := json.Marshal(task); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
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

					http.Error(w, err.Error(), http.StatusInternalServerError)
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
