package tarfs

import (
	"io"
	"io/fs"
	"path"
	"sort"
	"sync"
	"time"
)

type dir struct {
	name    string
	modTime time.Time
	entries map[string]struct{}
}

func (d *dir) open(fileSystem *fileSystem) (fs.File, error) {
	return &openDir{fs: fileSystem, dir: d, entries: d.readDirNames()}, nil
}

func (d *dir) stat() fs.FileInfo {
	return dirInfo{d}
}

func (d *dir) readDirNames() []string {
	names := make([]string, 0, len(d.entries))
	for name := range d.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (d *dir) readDir(fileSystem *fileSystem) ([]fs.DirEntry, error) {
	return readDirEntries(fileSystem, d.readDirNames())
}

func readDirEntries(fileSystem *fileSystem, names []string) ([]fs.DirEntry, error) {
	entries := make([]fs.DirEntry, len(names))
	for i, name := range names {
		entries[i] = fs.FileInfoToDirEntry(fileSystem.files[name].stat())
	}
	return entries, nil
}

type dirInfo struct{ *dir }

func (info dirInfo) Name() string       { return path.Base(info.name) }
func (info dirInfo) Size() int64        { return 0 }
func (info dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0755 }
func (info dirInfo) ModTime() time.Time { return info.modTime }
func (info dirInfo) IsDir() bool        { return true }
func (info dirInfo) Sys() any           { return nil }

type openDir struct {
	mutex   sync.Mutex
	fs      *fileSystem
	dir     *dir
	entries []string
}

func (d *openDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{"read", d.dir.stat().Name(), fs.ErrInvalid}
}

func (d *openDir) Stat() (fs.FileInfo, error) {
	return d.dir.stat(), nil
}

func (d *openDir) Close() error {
	d.mutex.Lock()
	d.entries = nil
	d.mutex.Unlock()
	return nil
}

func (d *openDir) ReadDir(n int) (entries []fs.DirEntry, err error) {
	if names := d.readDir(n); len(names) > 0 {
		entries, err = readDirEntries(d.fs, names)
	} else if n > 0 {
		err = io.EOF
	}
	return entries, err
}

func (d *openDir) readDir(n int) (entries []string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if n < 0 || n >= len(d.entries) {
		entries, d.entries = d.entries, nil
	} else {
		entries, d.entries = d.entries[:n], d.entries[n:]
	}

	return entries
}

var (
	_ fs.ReadDirFile = (*openDir)(nil)
)
