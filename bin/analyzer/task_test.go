package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/containers/image/v5/storage"
	internal "github.com/dcermak/container-layer-sizes/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var imageTag = "localhost/container-layer-sizes-testimage"

var rootDir, subDir internal.Dir

func TestMain(m *testing.M) {
	err := reexecForRootlessStorage()
	if err != nil {
		panic(err)
	}

	code := m.Run()

	// os.Exit bypasses defer!
	os.Exit(code)
}

func TestNewTaskWithoutTransport(t *testing.T) {
	img := "docker.io/library/node"

	task, err := NewTask(img, "")
	require.NoErrorf(t, err, "Task should have been created but got %s", err)
	require.NotNil(t, task, "Task must not be nil")
	defer task.Cleanup()

	assert.Equal(t, TaskStateToStr(task.State), TaskStateToStr(TaskStateNew))

	assert.Equal(t, img, task.Image.Image)
	assert.Equal(t, "latest", task.Image.Tag)
	assert.Equal(t, "docker", task.Image.Transport)
}

func TestNewTaskWithDockerTransport(t *testing.T) {
	transport := "docker"
	img := "docker.io/library/node"

	task, err := NewTask(fmt.Sprintf("%s://%s", transport, img), "")
	require.NoErrorf(t, err, "Task should have been created but got %s", err)
	require.NotNil(t, task, "Task must not be nil")
	defer task.Cleanup()

	assert.Equal(t, TaskStateToStr(task.State), TaskStateToStr(TaskStateNew))

	assert.Equal(t, img, task.Image.Image)
	assert.Equal(t, "latest", task.Image.Tag)
	assert.Equal(t, "docker", task.Image.Transport)

	ref, err := storage.Transport.ParseReference(img)
	require.NoErrorf(t, err, "creating the storage reference to %s resulted in the error %s", img, err)
	assert.Equal(t, ref, task.Image.localReference)
}

func TestNewTaskWithTransport(t *testing.T) {
	transport := "containers-storage"
	tag := "3.0"

	task, err := NewTask(fmt.Sprintf("%s:%s:%s", transport, imageTag, tag), imageTag)
	require.NoErrorf(t, err, "Task should have been created but got %s", err)
	require.NotNil(t, task, "Task must not be nil")
	defer task.Cleanup()

	assert.Equal(t, TaskStateToStr(task.State), TaskStateToStr(TaskStateNew))

	assert.Equal(t, imageTag, task.Image.Image)
	assert.Equal(t, tag, task.Image.Tag)
	assert.Equal(t, transport, task.Image.Transport)
}

func TestNewTaskWithInvalidImageName(t *testing.T) {
	task, err := NewTask("", "")
	assert.Error(t, err, "Expected to receive an error, but got none")
	assert.Nil(t, task)

	task, err = NewTask("docker://docker.io/library/golang:1.16:foobar", "")
	assert.Error(t, err, "Expected to receive an error, but got none")
	assert.Nil(t, task)
}

type ImageSizeSuite struct {
	suite.Suite
	imagePath   string
	expectedTag string
	imageName   string
	task        *Task
}

func (s *ImageSizeSuite) SetupSuite() {
	var err error
	s.task, err = NewTask(s.imagePath, s.imageName)
	require.NoErrorf(s.T(), err, "Expected to create the task with the image %s, but got %s", s.imagePath, err)
}

func (s *ImageSizeSuite) TearDownSuite() {
	err := s.task.Cleanup()
	require.NoErrorf(s.T(), err, "Expected to successfully cleanup the task, but got %s", err)
}

func (s *ImageSizeSuite) TestCreatedSizes() {
	if s.task.State != TaskStateNew {
		s.T().Errorf("Got a wrong task state, expected %d, but got %d", TaskStateNew, s.task.State)
	}
	s.task.Process()
	assert.NoError(s.T(), s.task.error)

	layerCount := 0
	layerDigest := ""
	for digest, _ := range *s.task.Image.layers {
		layerDigest = digest
		layerCount += 1
	}

	assert.Equal(s.T(), 1, layerCount)
	createdBy := (*s.task.Image.layers)[layerDigest].CreatedBy

	if s.expectedTag == "3.0" {
		assert.Equal(s.T(), internal.Layer{
			Dir:       rootDir,
			CreatedBy: createdBy,
		}, (*s.task.Image.layers)[layerDigest])
	} else {
		assert.Equal(s.T(), internal.Layer{
			Dir:       subDir,
			CreatedBy: createdBy,
		}, (*s.task.Image.layers)[layerDigest])
	}

}

func (s *ImageSizeSuite) TestImageMetadata() {
	assert.Equal(s.T(), s.expectedTag, s.task.Image.Tag)
}

func TestIntegration(t *testing.T) {
	dir := os.Getenv("IMG_DIR")
	if dir == "" {
		panic("environment variable IMG_DIR is not set")
	}

	rootDir = internal.MakeDir("/")
	subDir = internal.MakeDir("/")

	for i := 1; i <= 100; i++ {
		fname := fmt.Sprintf("/%d", i)
		fsize := int64(i * 512)
		rootDir.InsertIntoDir(fname, fsize)
		rootDir.InsertIntoDir(fmt.Sprintf("/subdir/%d", i), fsize*2)
		subDir.InsertIntoDir(fname, fsize*2)
	}

	for _, tag := range []string{"3.0", "latest"} {
		s := *new(ImageSizeSuite)
		s.expectedTag = tag
		s.imagePath = strings.ReplaceAll(
			fmt.Sprintf("docker-archive:%s/%s/testimage.tar.gz", dir, tag),
			"//", "/",
		)
		suite.Run(t, &s)

		for _, img := range []string{
			fmt.Sprintf("oci:%s/%s/testimage:%s", dir, tag, tag),
			fmt.Sprintf("containers-storage:%s:%s", imageTag, tag),
		} {
			s := *new(ImageSizeSuite)
			s.expectedTag = tag
			s.imagePath = img
			s.imageName = imageTag
			suite.Run(t, &s)
		}
	}
}
