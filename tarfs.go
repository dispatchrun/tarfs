package tarfs

import (
	"archive/tar"
	"errors"
	"io"
	"io/fs"
	"path"
	"time"

	"github.com/stealthrocket/fslink"
)

var (
	ErrLoop = errors.New("tarfs: loop detected while following symbolic links")
)

func OpenFS(data io.ReaderAt, size int64) (fs.FS, error) {
	modTime := time.Now()
	section := io.NewSectionReader(data, 0, size)
	reader := tar.NewReader(section)
	links := []*link{}
	files := map[string]fileEntry{
		".": &dir{name: ".", modTime: modTime}, // root
	}

	err := walk(reader, func(header *tar.Header) error {
		var entry fileEntry

		switch header.Typeflag {
		case tar.TypeReg:
			offset, _ := section.Seek(0, io.SeekCurrent)
			entry = &file{header: header, offset: offset}

		case tar.TypeDir:
			entry = &dir{name: header.Name, modTime: modTime}

		case tar.TypeLink:
			ln := &link{header: header}
			entry = ln
			links = append(links, ln)

		case tar.TypeSymlink:
			entry = symlink{header}

		default:
			entry = deny{header}
		}

		makePath(files, header.Name, modTime)
		files[header.Name] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, ln := range links {
		ln.target, _ = files[ln.header.Linkname].(*file)
	}

	fileSystem := &fileSystem{
		data:  data,
		size:  size,
		files: files,
	}

	return fileSystem, nil
}

func walk(r *tar.Reader, f func(*tar.Header) error) error {
	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}

		// ensure that no path will reference parent directories above the root
		h.Name = path.Join("/", h.Name)
		if h.Name == "/" {
			continue // don't allow overriding the root
		}
		h.Name = h.Name[1:] // strip leading "/"

		if err := f(h); err != nil {
			return err
		}
	}
}

const (
	maxFollowSymlink = 40
)

type fileSystem struct {
	data  io.ReaderAt
	size  int64
	files map[string]fileEntry
}

type fileEntry interface {
	open(*fileSystem) (fs.File, error)
	stat() fs.FileInfo
}

func (f *fileSystem) Open(name string) (fs.File, error) {
	for i := 0; i < maxFollowSymlink; i++ {
		entry, err := f.lookup(name)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		if s, ok := entry.(symlink); ok {
			name = s.target()
			continue
		}
		return entry.open(f)
	}
	return nil, ErrLoop
}

func (f *fileSystem) Stat(name string) (fs.FileInfo, error) {
	for i := 0; i < maxFollowSymlink; i++ {
		entry, err := f.lookup(name)
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
		}
		if s, ok := entry.(symlink); ok {
			name = s.target()
			continue
		}
		return entry.stat(), nil
	}
	return nil, ErrLoop
}

func (f *fileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := f.readDir(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	return entries, nil
}

func (f *fileSystem) ReadLink(name string) (string, error) {
	link, err := f.readLink(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	return link, nil
}

func (f *fileSystem) lookup(name string) (fileEntry, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	entry, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return entry, nil
}

func (f *fileSystem) readDir(name string) ([]fs.DirEntry, error) {
	entry, err := f.lookup(name)
	if err != nil {
		return nil, err
	}
	d, ok := entry.(*dir)
	if !ok {
		return nil, fs.ErrPermission
	}
	return d.readDir(f)
}

func (f *fileSystem) readLink(name string) (string, error) {
	entry, err := f.lookup(name)
	if err != nil {
		return "", err
	}
	s, ok := entry.(symlink)
	if !ok {
		return "", fs.ErrInvalid
	}
	return s.header.Linkname, nil
}

func makePath(files map[string]fileEntry, name string, modTime time.Time) error {
	var d *dir

	dirname := path.Dir(name)
	switch f := files[dirname].(type) {
	case nil:
		if err := makePath(files, dirname, modTime); err != nil {
			return err
		}
		d = &dir{name: dirname, modTime: modTime}
		files[dirname] = d
	case *dir:
		d = f
	default:
		return fs.ErrPermission
	}

	if d.entries == nil {
		d.entries = make(map[string]struct{})
	}
	d.entries[name] = struct{}{}
	return nil
}

var (
	_ fs.ReadDirFS      = (*fileSystem)(nil)
	_ fs.StatFS         = (*fileSystem)(nil)
	_ fslink.ReadLinkFS = (*fileSystem)(nil)
)
