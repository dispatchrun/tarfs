package tarfs_test

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"os"
	"testing"

	"github.com/stealthrocket/tarfs"
	"github.com/stealthrocket/fstest"
)

func TestExtractArchive(t *testing.T) {
	fsys := fstest.MapFS{
		"var":                &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		"var/run":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		"var/log":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		"var/log/system.log": &fstest.MapFile{Mode: 0600, Data: []byte("hello world!")},
	}

	buffer := new(bytes.Buffer)
	writer := tar.NewWriter(buffer)

	if err := tarfs.Archive(writer, fsys); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	reader := tar.NewReader(buffer)

	if err := tarfs.Extract(tmp, reader); err != nil {
		t.Fatal(err)
	}
	if err := fstest.EqualFS(os.DirFS(tmp), fsys); err != nil {
		t.Fatal(err)
	}
}
