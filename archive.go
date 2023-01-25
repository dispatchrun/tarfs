package tarfs

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"

	"github.com/stealthrocket/fsinfo"
	"github.com/stealthrocket/fslink"
)

// Archive archives a file system into a tarball.
//
// If the file system contains symbolic links, it must implement a ReadLink
// method with this signature to allow reading the value of the link targets:
//
//	type ReadLinkFS interface {
//		ReadLink(name string) (string, error)
//	}
//
// See https://github.com/golang/go/issues/49580 for details about the expected
// behavior of the ReadLinkFS interface.
func Archive(tarball *tar.Writer, fsys fs.FS) error {
	links := make(map[uint64]string)
	buffer := make([]byte, 32*1024)

	return fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()

		h := tar.Header{
			Name:    path,
			Mode:    int64(fsinfo.Mode(info)),
			ModTime: info.ModTime(),
			Format:  tar.FormatPAX,
		}

		switch mode.Type() {
		case 0: // regular
			h.Typeflag = tar.TypeReg

		case fs.ModeDir:
			h.Typeflag = tar.TypeDir

		case fs.ModeSymlink:
			s, err := fslink.ReadLink(fsys, path)
			if err != nil {
				return err
			}
			h.Typeflag = tar.TypeSymlink
			h.Linkname = s

		case fs.ModeNamedPipe:
			h.Typeflag = tar.TypeFifo

		case fs.ModeDevice:
			h.Typeflag = tar.TypeBlock

		case fs.ModeDevice | fs.ModeCharDevice:
			h.Typeflag = tar.TypeChar

		default:
			return nil // ignore unsupported file types
		}

		if !mode.IsDir() {
			if nlink := fsinfo.Nlink(info); nlink > 1 {
				if ino := fsinfo.Ino(info); ino != 0 {
					if link, ok := links[ino]; ok {
						h.Typeflag = tar.TypeLink
						h.Linkname = link
					} else {
						links[ino] = path
					}
				}
			}
		}

		switch h.Typeflag {
		case tar.TypeReg, tar.TypeChar, tar.TypeBlock:
			h.Size = info.Size()
		}

		if err := tarball.WriteHeader(&h); err != nil {
			return &fs.PathError{"write", path, err}
		}

		switch h.Typeflag {
		case tar.TypeReg, tar.TypeChar, tar.TypeBlock:
			file, err := fsys.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			n, err := io.CopyBuffer(tarball, file, buffer)
			if err != nil {
				return err
			}
			if size := info.Size(); size != n {
				err := fmt.Errorf("file size and number of bytes written mismatch: size=%d written=%d", size, n)
				return &fs.PathError{"write", path, err}
			}
		}

		return nil
	})
}
