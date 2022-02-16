package internal

import (
	"os"
	"path"
	"strings"
)

/// A structure to store the file sizes of a directory including all subdirectories.
type Dir struct {
	/// The name of this directory. Must not include any `/`.
	DirName string `json:"dirname"`

	/// Total size of this directory including all of its subdirectories in bytes
	TotalSize int64 `json:"total_size"`

	/// Map of all files in this immediate directory (i.e. not in subdirectories).
	///
	/// Each file is a single entry in the map and the corresponding value is its size in bytes
	Files map[string]int64 `json:"files"`

	/// Map of all immediate subdirectories of this directory.
	Directiories map[string]Dir `json:"directories"`
}

/// Creates an empty directory with the given directory name `dirname`.
func MakeDir(dirname string) Dir {
	return Dir{DirName: dirname, TotalSize: 0, Files: make(map[string]int64), Directiories: make(map[string]Dir)}
}

/// Returns a string slice without all empty strings.
/// !Caution! This modifies `sl` in-place!
func dropEmptyStrings(sl []string) []string {
	i := 0
	for _, s := range sl {
		if len(s) > 0 {
			sl[i] = s
			i++
		}
	}
	sl = sl[:i]
	return sl
}

/// Insert the file with the given path `filePath` into the directory structure starting at `d`.
///
/// Any non-existing directories leading up to `filePath` are created and the file is inserted into the correct spot.
/// The total size of all directories is adjusted accordingly.
func (d *Dir) InsertIntoDir(filePath string, size int64) {
	fname := path.Base(filePath)
	dirname := path.Dir(filePath)

	dirs := dropEmptyStrings(strings.Split(dirname, string(os.PathSeparator)))

	d.TotalSize += size

	if dirname == path.Base(d.DirName) || dirname == "." {
		d.Files[fname] = size
	} else {
		if _, ok := d.Directiories[dirs[0]]; !ok {
			d.Directiories[dirs[0]] = MakeDir(dirs[0])
		}
		subdir := d.Directiories[dirs[0]]
		subdir.InsertIntoDir(strings.Join(append(dirs[1:], fname), string(os.PathSeparator)), size)
		d.Directiories[dirs[0]] = subdir
	}
}
