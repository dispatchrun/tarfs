package tarfs_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/tarfs"
)

func TestTarFS(t *testing.T) {
	t.Run("fstest", func(t *testing.T) {
		buffer := bytes.NewBuffer(nil)
		writer := tar.NewWriter(buffer)

		writeFile(t, writer, "/entrypoint.wasm", "<!>", 0755)
		writeFile(t, writer, "./file-0", "Hello World!", 0644)
		writeFile(t, writer, "sub/file-1", "123", 0644)
		writeFile(t, writer, "sub/file-2", "456", 0644)
		writeFile(t, writer, "sub/file-3", "789", 0644)
		writeDir(t, writer, "empty")
		writeDir(t, writer, "/tmp")
		writeFile(t, writer, "tmp/subtmp/one", "1", 0644)
		writeFile(t, writer, "tmp/subtmp/two", "2", 0644)
		writeLink(t, writer, "tmp/link-0", "file-0")
		writeSymlink(t, writer, "tmp/symlink-0", "../sub/file-1")
		writeSymlink(t, writer, "tmp/symlink-1", "../sub/file-2")
		closeArchive(t, writer)

		fileSystem := openFS(t, buffer.Bytes())

		if err := fstest.TestFS(fileSystem,
			"file-0",
			"sub",
			"sub/file-1",
			"sub/file-2",
			"sub/file-3",
			"empty",
			"tmp",
			"tmp/subtmp/one",
			"tmp/subtmp/two",
			"tmp/link-0",
			"tmp/symlink-0",
			"tmp/symlink-1",
		); err != nil {
			t.Fatal(err)
		}

		assertReadFile(t, fileSystem, "entrypoint.wasm", "<!>")
		assertReadFile(t, fileSystem, "file-0", "Hello World!")
		assertReadFile(t, fileSystem, "tmp/link-0", "Hello World!")
		assertReadFile(t, fileSystem, "tmp/symlink-0", "123") // follow
	})

	t.Run("permission denied", func(t *testing.T) {
		buffer := bytes.NewBuffer(nil)
		writer := tar.NewWriter(buffer)

		writeBlock(t, writer, "tmp/block")
		closeArchive(t, writer)

		fileSystem := openFS(t, buffer.Bytes())
		assertPermissionDenied(t, fileSystem, "tmp/block")
	})
}

func openFS(t *testing.T, data []byte) fs.FS {
	fileSystem, err := tarfs.OpenFS(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return fileSystem
}

func assertReadFile(t *testing.T, f fs.FS, name, content string) {
	b, err := fs.ReadFile(f, name)
	if err != nil {
		t.Error(err)
	} else if string(b) != content {
		t.Errorf("content of %s mismatch: got=%q want=%q", name, b, content)
	}
}

func assertPermissionDenied(t *testing.T, f fs.FS, name string) {
	_, err := fs.ReadFile(f, name)
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error reading %s mismatch: %v", name, err)
	}
}

func writeDir(t *testing.T, w *tar.Writer, name string) {
	t.Helper()
	if err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     name,
		Mode:     0755,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, w *tar.Writer, name, content string, mode int64) {
	t.Helper()
	if err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     mode,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := io.WriteString(w, content)
	if err != nil {
		t.Fatal(err)
	}
}

func writeLink(t *testing.T, w *tar.Writer, name, link string) {
	t.Helper()
	if err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeLink,
		Name:     name,
		Linkname: link,
		Mode:     0644,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeSymlink(t *testing.T, w *tar.Writer, name, link string) {
	t.Helper()
	if err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     name,
		Linkname: link,
		Mode:     0777,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeBlock(t *testing.T, w *tar.Writer, name string) {
	t.Helper()
	if err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeBlock,
		Name:     name,
		Mode:     0644,
	}); err != nil {
		t.Fatal(err)
	}
}

func closeArchive(t *testing.T, w *tar.Writer) {
	t.Helper()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
