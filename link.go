package tarfs

import (
	"archive/tar"
	"io/fs"
)

type link struct {
	target *file
	header *tar.Header
}

func (ln *link) open(fileSystem *fileSystem) (fs.File, error) {
	if ln.target == nil {
		return nil, fs.ErrNotExist
	}
	return ln.target.openFile(fileSystem, ln.header), nil
}

func (ln *link) stat() fs.FileInfo {
	return ln.header.FileInfo()
}
