package internal

import (
	"github.com/containers/image/v5/types"
)

type Layer struct {
	Dir

	/// The command that was used to create this layer
	CreatedBy string
}

func NewLayer() Layer {
	d := MakeDir("/")
	return Layer{Dir: d}
}

/// The size tree of all layers of a container image
/// The key is the hash of each layer, the value is the calculated directory sizes
type LayerSizes map[string]Layer

/// A single entry in the history of an image
type ImageHistoryEntry struct {
	// database primary key
	id int64

	Tags        []string
	Contents    LayerSizes
	InspectInfo types.ImageInspectInfo
}

type ImageEntry struct {
	// database primary key
	ID int64

	/// The name of this image
	Name string
}

/// The full image history.
type ImageHistory struct {
	ImageEntry

	/// map of the container hashes and the corresponding entry
	History map[string]ImageHistoryEntry
}

type StorageBackend interface {
	Migrate() error
	Read(imageName string) ([]ImageHistory, error)
	Create(imageHistory ImageHistory) (*ImageHistory, error)
	Update(imageHistory *ImageHistory) (*ImageHistory, error)
	Delete(imageHistory *ImageHistory) error
}
