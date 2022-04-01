package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

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

type ImageParseSuite struct {
	suite.Suite
	imageUrl          string
	expectedTag       string
	expectedName      string
	expectedTransport string
	task              *Task
}

func (s *ImageParseSuite) SetupSuite() {
	var err error
	s.task, err = NewTask(s.imageUrl)
	require.NoError(s.T(), err)
}
func (s *ImageParseSuite) TearDownSuite() {
	err := s.task.Cleanup()
	require.NoError(s.T(), err)
}

func (s *ImageParseSuite) TestImageName() {
	assert.Equal(s.T(), s.expectedName, s.task.Image.Image)
}

func (s *ImageParseSuite) TestImageTag() {
	assert.Equal(s.T(), s.expectedTag, s.task.Image.Tag)
}

func (s *ImageParseSuite) TestImageTransport() {
	assert.Equal(s.T(), s.expectedTransport, s.task.Image.Transport)
}

func (s *ImageParseSuite) TestReferencesAreValid() {
	assert.NotNil(s.T(), s.task.Image.remoteReference)
	assert.NotNil(s.T(), s.task.Image.localReference)
	assert.NotNil(s.T(), s.task.Image.ociLocalReference)
}

func (s *ImageParseSuite) TestStateIsNew() {
	assert.Equal(s.T(), TaskStateToStr(s.task.State), TaskStateToStr(TaskStateNew))
}

func TestImageParsing(t *testing.T) {
	for _, transport := range []string{"containers-storage", "docker"} {

		transportStr := transport + ":"
		expectedTransport := transport
		if transport == "docker" {
			transportStr = "docker://"
		}

		// the images references here must exist on your system!
		// if changing anything here, be sure to update ./test_data/setup_images.sh
		imgTests := []ImageParseSuite{
			ImageParseSuite{
				imageUrl:          fmt.Sprintf("%sdocker.io/library/alpine", transportStr),
				expectedName:      "docker.io/library/alpine",
				expectedTag:       "latest",
				expectedTransport: expectedTransport,
			},
			ImageParseSuite{
				imageUrl:          fmt.Sprintf("%sdocker.io/library/alpine:3.15", transportStr),
				expectedName:      "docker.io/library/alpine",
				expectedTag:       "3.15",
				expectedTransport: expectedTransport,
			},
			ImageParseSuite{
				imageUrl:          fmt.Sprintf("%sdocker.io/library/alpine:edge", transportStr),
				expectedName:      "docker.io/library/alpine",
				expectedTag:       "edge",
				expectedTransport: expectedTransport,
			},
		}

		for _, iT := range imgTests {
			suite.Run(t, &iT)
		}
	}
}

func TestNewTaskWithInvalidImageName(t *testing.T) {
	task, err := NewTask("")
	assert.Error(t, err, "Expected to receive an error, but got none")
	assert.Nil(t, task)

	task, err = NewTask("docker://docker.io/library/golang:1.16:foobar")
	assert.Error(t, err, "Expected to receive an error, but got none")
	assert.Nil(t, task)
}

type ImageSizeSuite struct {
	suite.Suite
	imagePath   string
	expectedTag string
	task        *Task
}

func (s *ImageSizeSuite) SetupSuite() {
	var err error
	s.task, err = NewTask(s.imagePath)
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
			// FIXME: re-enable oci image support again
			// fmt.Sprintf("oci:%s/%s/testimage:%s", dir, tag, tag),
			fmt.Sprintf("containers-storage:%s:%s", imageTag, tag),
		} {
			s := *new(ImageSizeSuite)
			s.expectedTag = tag
			s.imagePath = img
			suite.Run(t, &s)
		}
	}
}
