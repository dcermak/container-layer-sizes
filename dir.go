package main

import (
	"os"
	"path"
	"strings"
)

type Dir struct {
	TotalSize    int64            `json:"total_size"`
	Files        map[string]int64 `json:"files"`
	Directiories map[string]Dir   `json:"directories"`
	DirName string `json:"dirname"`
}

func MakeDir(dirname string) Dir {
	return Dir{DirName: dirname, TotalSize: 0, Files: make(map[string]int64), Directiories: make(map[string]Dir)}
}

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
