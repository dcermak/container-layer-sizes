package main

import (
	"testing"
)

func TestMakeDir(t *testing.T) {
	root := MakeDir("/")
	if root.TotalSize != 0 {
		t.Errorf("invalid totalSize %d, expected 0", root.TotalSize)
	}
	if root.DirName != "/" {
		t.Errorf("Invalid path '%s', expected '/'", root.DirName)
	}
}

func TestInsertFile(t *testing.T) {
	root := MakeDir("/")

	var size int64 = 42

	root.InsertIntoDir("/foo", size)

	if len(root.Directiories) != 0 {
		t.Errorf("No directory entries must be in root, but got %d", len(root.Directiories))
	}

	if fooSize, ok := root.Files["foo"]; !ok {
		t.Error("Expected files to have the 'foo' entry")
	} else if fooSize != size {
		t.Errorf("Expected size of foo to equal %d but got %d", size, fooSize)
	}

	if root.TotalSize != size {
		t.Errorf("/ has invalid total size, expected: %d, got: %d", size, root.TotalSize)
	}
}

func TestInsertSubdir(t *testing.T) {
	root := MakeDir("/")

	var size int64 = 42

	root.InsertIntoDir("/foo/bar", size)

	if len(root.Directiories) != 1 {
		t.Errorf("One directory entries must be in root, but got %d", len(root.Directiories))
	}

	if fooDir, ok := root.Directiories["foo"]; !ok {
		t.Error("Expected directories to have the 'foo' entry")
	} else {
		if l := len(fooDir.Directiories); l != 0 {
			t.Errorf("Expected no directories in subdir 'foo', but got %d", l)
		}
		if fooDir.DirName != "foo" {
			t.Errorf("Invalid path of foo subdirectory: %s", fooDir.DirName)
		}

		if fooDir.TotalSize != size {
			t.Errorf("Invalid total size of /foo: %d, expected %d", fooDir.TotalSize, size)
		}

		if barFile, barPresent := fooDir.Files["bar"]; !barPresent {
			t.Error("File 'bar' not present in 'foo'")
		} else if barFile != size {
			t.Errorf("File 'bar' has invalid size %d, expected %d", barFile, size)
		}
	}
}

func TestRecursiveInsert(t *testing.T) {
	root := MakeDir("/")

	var fSize int64 = 16
	p := "/level1/level2/file"

	root.InsertIntoDir(p, fSize)

	lvl1 := root.Directiories["level1"]
	lvl2 := lvl1.Directiories["level2"]
	if gotSize := lvl2.Files["file"]; gotSize != fSize {
		t.Errorf("Invalid size of %s: %d, expected %d", p, gotSize, fSize)
	}
}

func TestInsertMultiple(t *testing.T) {
	root := MakeDir("/")

	cat := "/usr/bin/cat"
	libdl := "/usr/lib64/dl"
	osRelease := "/etc/os-release"

	catSize := int64(16)
	libdlSize := int64(48)
	osReleaseSize := int64(5)

	usrSize := catSize + libdlSize
	rootSize := usrSize + osReleaseSize

	root.InsertIntoDir(cat, catSize)
	root.InsertIntoDir(libdl, libdlSize)
	root.InsertIntoDir(osRelease, osReleaseSize)

	if root.TotalSize != rootSize {
		t.Errorf("Invalid total size of '/', expected %d, got %d", rootSize, root.TotalSize)
	}

	if uS := root.Directiories["usr"].TotalSize; uS != usrSize {
		t.Errorf("Invalid total size of /usr, got: %d, expected: %d", uS, usrSize)
	}

	if eS := root.Directiories["etc"].TotalSize; eS != osReleaseSize {
		t.Errorf("Invalid total size of /etc, got: %d, expected: %d", eS, osReleaseSize)
	}
}
