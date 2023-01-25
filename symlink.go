package tarfs

import (
	"archive/tar"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync"
)

type symlink struct{ header *tar.Header }

func (ln symlink) open(fileSystem *fileSystem) (fs.File, error) {
	f := &openSymlink{header: ln.header}
	f.reader.Reset(ln.header.Linkname)
	return f, nil
}

func (ln symlink) stat() fs.FileInfo {
	return ln.header.FileInfo()
}

func (ln symlink) target() string {
	return path.Join(path.Dir(ln.header.Name), ln.header.Linkname)
}

type openSymlink struct {
	mutex  sync.RWMutex
	header *tar.Header
	reader strings.Reader
}

func (f *openSymlink) ReadAt(b []byte, offset int64) (int, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.reader.ReadAt(b, offset)
}

func (f *openSymlink) Read(b []byte) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.reader.Read(b)
}

func (f *openSymlink) Seek(offset int64, whence int) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.reader.Seek(offset, whence)
}

func (f *openSymlink) Close() error {
	f.mutex.Lock()
	f.reader.Reset("")
	f.mutex.Unlock()
	return nil
}

func (f *openSymlink) Stat() (fs.FileInfo, error) {
	return f.header.FileInfo(), nil
}

var (
	_ fs.File     = (*openSymlink)(nil)
	_ io.ReaderAt = (*openSymlink)(nil)
	_ io.Seeker   = (*openSymlink)(nil)
)
