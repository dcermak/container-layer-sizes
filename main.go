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
	archiver "github.com/mholt/archiver/v3"
	"github.com/syndtr/gocapability/capability"
)

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

func PullImageToLocalStorage(image string) error {
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

	if _, err := copy.Image(backgroundContext, policyCtx, localRef, remoteRef, &copy.Options{
		//DestinationCtx: types.SystemContext{},
	}); err != nil {
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

func CalculateContainerLayerSizes(unpackedImageDest string, manifest []byte) (LayerSizes, error) {
	var result map[string]interface{}
	json.Unmarshal(manifest, &result)

	layers := make(LayerSizes)

	for _, layer := range result["layers"].([]interface{}) {
		mediatype := layer.(map[string]interface{})["mediaType"]
		if mediatype != "application/vnd.docker.image.rootfs.diff.tar.gzip" {
			return nil, errors.New(fmt.Sprintf("Invalid media type: %s", mediatype))
		}

		digest := strings.Split(layer.(map[string]interface{})["digest"].(string), ":")
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

	http.HandleFunc("/data/", func(w http.ResponseWriter, r *http.Request) {
		image := r.URL.Path[6:]
		if err = PullImageToLocalStorage(image); err != nil {
			panic(err)
		}

		dir, err := ioutil.TempDir("", "")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)

		manifest, err := CopyImageIntoDest(image, dir)
		if err != nil {
			panic(err)
		}

		layers, err := CalculateContainerLayerSizes(dir, manifest)
		if j, err := json.MarshalIndent(layers, "", "  "); err != nil {
			panic(err)
		} else {
			fmt.Fprint(w, string(j))
		}
	})
	if err := http.ListenAndServe(":5050", nil); err != nil {
		panic(err)
	}
}
