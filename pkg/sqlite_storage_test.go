package internal

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var s *SQLiteBackend

var entryOne, entryTwo ImageHistoryEntry

func TestMain(m *testing.M) {

	layerOne := "asdf"
	layerTwo := "uiae"
	l := NewLayer()
	l.InsertIntoDir("/etc/os-release", 128)
	entryOne = ImageHistoryEntry{
		id:   0,
		Tags: []string{"latest", "1.0"},
		Contents: map[string]Layer{
			"layerOne": NewLayer(),
			"layerTwo": l,
		},
		InspectInfo: types.ImageInspectInfo{
			DockerVersion: "1.22",
			Labels: map[string]string{
				"org.opencontainers.image.vendor":  "ME!",
				"org.opencontainers.image.version": "1.0",
			},
			Architecture: "x86_64",
			Os:           "Linux",
			Layers:       []string{layerOne, layerTwo},
			Env:          []string{},
		},
	}

	file, err := ioutil.TempFile("", "testDb.*.sqlite3")
	if err != nil {
		panic(err)
	}
	defer os.Remove(file.Name())

	if s, err = CreateSQLiteBackend(file.Name()); err != nil {
		panic(err)
	}
	code := m.Run()

	if err = s.Destroy(); err != nil {
		panic(err)
	}

	// os.Exit bypasses defer
	os.Remove(file.Name())
	os.Exit(code)
}

func TestSimpleCreate(t *testing.T) {
	h := &ImageHistory{}
	h.Name = "foobar"

	h2, err := s.Create(h)
	require.Nilf(t, err, "Failed to create empty image history, got %s", err)

	assert.GreaterOrEqualf(t, h2.ID, int64(1), "Created image history got an invalid id %d", h2.ID)
	assert.Equalf(t, h2.Name, h.Name, "Created ImageHistory has a different name, expected %s, but got %s", h.Name, h2.Name)
}

func TestSimpleRead(t *testing.T) {
	h := &ImageHistory{}
	h.Name = "test"
	_, err := s.Create(h)
	require.Nilf(t, err, "Failed to create image history entry with name %s, got %s", h.Name, err)

	h2, err := s.Read(h.Name)
	require.Nilf(t, err, "Failed to find the image history entries with name %s, got %s", h.Name, err)

	foundRows := len(h2)
	assert.Equalf(t, foundRows, 1, "Expected to find one row, but got %d", foundRows)

	assert.Equalf(t, h.Name, h2[0].Name, "Inserted and found image history do not match %v != %v", h, h2[0])

}

func TestComplexRead(t *testing.T) {
	h := &ImageHistory{
		History: map[string]ImageHistoryEntry{
			"foobar": entryOne,
			"bazBar": entryTwo,
		},
	}
	h.Name = "complexImage"

	h, err := s.Create(h)
	require.Nilf(t, err, "Could not insert %v into the database, got %s", h, err)

	h2, err := s.Read(h.Name)
	require.Nilf(t, err, "Failed to find the image history entries with name %s, got %s", h.Name, err)

	require.Equal(t, len(h2), 1, "Expected to find one row")

	assert.Equal(t, *h, h2[0], "Written and read ImageHistory do not match")
}

func TestUpdate(t *testing.T) {
	h := &ImageHistory{History: map[string]ImageHistoryEntry{}}
	h.Name = "notYetComplex"

	h, err := s.Create(h)
	require.Nilf(t, err, "Could not insert %v into the database, got %s", h, err)

	h.History["asdfuiaeuiae"] = entryOne

	h, err = s.Update(h)
	require.Nilf(t, err, "Could not update %v in the database, got %s", h, err)

	h2, err := s.ReadById(h.ID)
	assert.Equal(t, h, h2, "written and updated ImageHistory do not match")
}

func TestReadAll(t *testing.T) {
	h1 := &ImageHistory{History: map[string]ImageHistoryEntry{"bar": entryOne, "baz": entryTwo}}
	h1.Name = "testEntry"
	h1, err := s.Create(h1)
	require.NoError(t, err)

	h2 := &ImageHistory{}
	h2.Name = "second_test_entry"
	h2, err = s.Create(h2)
	require.NoError(t, err)

	allEntries, err := s.ReadAll()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(allEntries), 2)

	findEntry := func(entry ImageHistory) bool {
		for _, e := range allEntries {
			if e.ID == entry.ID && e.Name == entry.Name {
				return true
			}
		}
		return false
	}

	assert.Truef(t, findEntry(*h1), "Expected to find the entry h1 in the database")
	assert.Truef(t, findEntry(*h2), "Expected to find the entry h2 in the database")
}
