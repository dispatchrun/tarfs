package tarfs

import (
	"archive/tar"
	"io/fs"
	"path"
	"time"
)

type deny struct{ header *tar.Header }

func (d deny) open(fileSystem *fileSystem) (fs.File, error) {
	return nil, fs.ErrPermission
}

func (d deny) stat() fs.FileInfo {
	return denyInfo{d.header}
}

type denyInfo struct{ header *tar.Header }

func (info denyInfo) Name() string       { return path.Base(info.header.Name) }
func (info denyInfo) Size() int64        { return info.header.Size }
func (info denyInfo) Mode() fs.FileMode  { return fs.FileMode(info.header.Mode & ^0777) }
func (info denyInfo) ModTime() time.Time { return info.header.ModTime }
func (info denyInfo) IsDir() bool        { return info.Mode().IsDir() }
func (info denyInfo) Sys() any           { return nil }
