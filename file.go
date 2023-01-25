package tarfs

import (
	"archive/tar"
	"io"
	"io/fs"
	"sync"
)

type file struct {
	header *tar.Header
	offset int64
}

func (f *file) openFile(fileSystem *fileSystem, header *tar.Header) *openFile {
	return &openFile{
		header: header,
		reader: io.NewSectionReader(fileSystem.data, f.offset, f.header.Size),
	}
}

func (f *file) open(fileSystem *fileSystem) (fs.File, error) {
	return f.openFile(fileSystem, f.header), nil
}

func (f *file) stat() fs.FileInfo {
	return f.header.FileInfo()
}

type openFile struct {
	mutex  sync.RWMutex
	header *tar.Header
	reader *io.SectionReader
}

func (f *openFile) ReadAt(b []byte, offset int64) (int, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	if f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.ReadAt(b, offset)
}

func (f *openFile) Read(b []byte) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.Read(b)
}

func (f *openFile) Seek(offset int64, whence int) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.reader == nil {
		return 0, fs.ErrClosed
	}
	return f.reader.Seek(offset, whence)
}

func (f *openFile) Close() error {
	f.mutex.Lock()
	f.reader = nil
	f.mutex.Unlock()
	return nil
}

func (f *openFile) Stat() (fs.FileInfo, error) {
	return f.header.FileInfo(), nil
}

var (
	_ fs.File     = (*openFile)(nil)
	_ io.ReaderAt = (*openFile)(nil)
	_ io.Seeker   = (*openFile)(nil)
)
