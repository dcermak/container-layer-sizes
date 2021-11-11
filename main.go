package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"archive/tar"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/unshare"

	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	archiver "github.com/mholt/archiver/v3"
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

type LayerSizes map[string]Dir

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

type Task struct {
	/// URL of the container image
	Image string    `json:"image"`
	State TaskState `json:"state"`

	/// Metadata of the container image
	Metadata string `json:"metadata"`

	/// an error if any occurred
	error error

	layers *LayerSizes

	tempdir string
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

func NewTask(image string) (*Task, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	layers := make(LayerSizes)

	return &Task{
			Image:   image,
			State:   TaskStateNew,
			layers:  &layers,
			tempdir: tempdir,
			error:   nil},
		nil
}

func (t *Task) Process() {
	setError := func(err error) {
		t.error = err
		t.State = TaskStateError
	}

	t.State = TaskStatePulling
	opts := copy.Options{
		// ProgressInterval: 1,
		// Progress:         make(chan types.ProgressProperties),
	}
	if err := PullImageToLocalStorage(t.Image, &opts); err != nil {
		setError(err)
		return
	}

	t.State = TaskStateExtracting
	m, err := CopyImageIntoDest(t.Image, t.tempdir)
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

	m, err = ReadImageMetadata(t.tempdir, manifest)
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

func (tq *TaskQueue) AddTask(image string) (string, *Task, error) {
	id := fmt.Sprint(uuid.New())

	if t, err := NewTask(image); err != nil {
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

/// Effectively just `podman pull $image`
func PullImageToLocalStorage(image string, opts *copy.Options) error {
	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return err
	}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}
	defer policyCtx.Destroy()

	localRef, err := storage.Transport.ParseReference(image)
	if err != nil {
		return err
	}

	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	remoteRef, err := docker.NewReference(reference.TagNameOnly(ref))
	if err != nil {
		return err
	}

	if _, err := copy.Image(backgroundContext, policyCtx, localRef, remoteRef, opts); err != nil {
		return err
	}
	return nil
}

func CopyImageIntoDest(image string, dest string) ([]byte, error) {
	ref, err := storage.Transport.ParseReference(image)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	img, err := ref.NewImage(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	destRef, err := directory.Transport.ParseReference("//" + dest)
	if err != nil {
		return nil, err
	}

	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return nil, err
	}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}

	manifest, err := copy.Image(ctx, policyCtx, destRef, ref, &copy.Options{RemoveSignatures: true})
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func ReadImageMetadata(unpackedImageDest string, manifest Manifest) ([]byte, error) {

	mediatype := manifest.Config.MediaType
	if mediatype != "application/vnd.docker.container.image.v1+json" {
		return nil, errors.New(fmt.Sprintf("Invalid media type: %s", mediatype))
	}

	digest := strings.Split(manifest.Config.Digest, ":")
	if len(digest) != 2 {
		return nil, errors.New(fmt.Sprintf("invalid digest: %s", digest))
	}

	return ioutil.ReadFile(filepath.Join(unpackedImageDest, digest[1]))

}

func CalculateContainerLayerSizes(unpackedImageDest string, manifest Manifest) (LayerSizes, error) {

	layers := make(LayerSizes)

	for _, layer := range manifest.Layers {
		mediatype := layer.MediaType
		if mediatype != "application/vnd.docker.image.rootfs.diff.tar.gzip" {
			return nil, errors.New(fmt.Sprintf("Invalid media type: %s", mediatype))
		}

		digest := strings.Split(layer.Digest, ":")
		if len(digest) != 2 {
			return nil, errors.New(fmt.Sprintf("invalid digest: %s", digest))
		}

		root := MakeDir("/")

		archivePath := filepath.Join(unpackedImageDest, digest[1])
		if err := archiver.NewTar().Walk(archivePath, func(f archiver.File) error {
			p := f.Header.(*tar.Header).Name
			if f.IsDir() {
				return nil
			}
			root.InsertIntoDir(p, f.Size())
			return nil
		}); err != nil {
			return nil, err
		}

		layers[digest[1]] = root
	}

	return layers, nil
}

func main() {
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

	fileServer := http.FileServer(http.Dir("./dist"))
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
			http.Error(w, err.Error(), 500)
		} else {
			fmt.Fprint(w, string(j))
		}
		tq.RemoveTask(id)
	})
	http.HandleFunc("/task", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Error parsing form data: %s", err), 400)
			return
		}

		switch r.Method {
		case "POST":
			image := r.PostFormValue("image")
			if image == "" {
				http.Error(w, "No image provided", 400)
				return
			}

			if id, t, err := tq.AddTask(image); err != nil {
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
				if err := tq.RemoveTask(id); err != nil {
					http.Error(w, err.Error(), 500)
				}
			}
			return
		}
	})
	if err := http.ListenAndServe(":5050", nil); err != nil {
		panic(err)
	}
}
