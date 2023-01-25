package tarfs

import (
	"archive/tar"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Extract extracts files from the tarbal to a directory at path on the file
// system.
//
// Note: at this time, since fs.FS is a read-only API, we chose to only support
// extracting files to a local path. This could be revisited in the future if Go
// gets an API to interact with writable file systems, likely we would then add
// a ExtractFS function to maintain backward compatiblity.
func Extract(path string, tarball *tar.Reader) error {
	unresolvedLinks := make(map[string]*tar.Header)
	buffer := make([]byte, 32*1024)
	directories := make([]*tar.Header, 0, 512)

	err := walk(tarball, func(h *tar.Header) error {
		fileName := filepath.FromSlash(h.Name)
		filePath := filepath.Join(path, fileName)

		if err := os.MkdirAll(filepath.Dir(filePath), 0777); err != nil {
			return err
		}

		mode := fs.FileMode(h.Mode).Perm()
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(filePath, 0777); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
			// We have to buffer all directories because we cannot set the
			// permissions until all the entries have been written or we could
			// be removing write permissions on a directory before its entries
			// are written.
			directories = append(directories, h)

		case tar.TypeFifo:
			// TODO: support creating named pipes
			return nil

		case tar.TypeSymlink:
			// TODO: test with relative and absolute link names
			// TODO: set times of symbolic link (Go has no API in the os package for that)
			if err := os.Symlink(h.Linkname, filePath); err != nil {
				if !errors.Is(err, fs.ErrExist) {
					return err
				}
			}

		case tar.TypeLink:
			linkName := filepath.FromSlash(h.Linkname)
			linkPath := filepath.Join(path, linkName)

			if err := os.Link(linkPath, filePath); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					unresolvedLinks[h.Linkname] = h
					return nil
				}
				return err
			}
			if err := chmodtimes(filePath, h); err != nil {
				return err
			}

		case tar.TypeChar:
			mode |= fs.ModeCharDevice
			fallthrough

		case tar.TypeBlock:
			mode |= fs.ModeDevice
			fallthrough

		case tar.TypeReg, tar.TypeRegA:
			if (mode & fs.ModeDevice) != 0 {
				// TODO: support creating devices
				// maj := int(h.Devmajor)
				// min := int(h.Devminor)
				return nil
			}
			f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			defer f.Close()
			if h.Size > 0 {
				if _, err := io.CopyBuffer(struct{ io.Writer }{f}, tarball, buffer); err != nil {
					return err
				}
			}
			if err := chtimes(filePath, h); err != nil {
				return err
			}
		}

		if link, ok := unresolvedLinks[h.Name]; ok {
			linkName := filepath.FromSlash(link.Name)
			linkPath := filepath.Join(path, linkName)

			if err := os.Link(filePath, linkPath); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
			if err := chmodtimes(linkPath, link); err != nil {
				return err
			}
			delete(unresolvedLinks, h.Name)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// There must be no unresolved links after the extraction completes since
	// there cannot be dangling hard links on a file system.
	for name := range unresolvedLinks {
		return &fs.PathError{Op: "link", Path: name, Err: fs.ErrNotExist}
	}

	for _, dir := range directories {
		dirName := filepath.FromSlash(dir.Name)
		dirPath := filepath.Join(path, dirName)
		if err := chmodtimes(dirPath, dir); err != nil {
			return err
		}
	}
	return nil
}

func chmodtimes(path string, file *tar.Header) error {
	if err := chmod(path, file); err != nil {
		return err
	}
	if err := chtimes(path, file); err != nil {
		return err
	}
	return nil
}

func chmod(path string, file *tar.Header) error {
	return os.Chmod(path, fs.FileMode(file.Mode).Perm())
}

func chtimes(path string, file *tar.Header) error {
	return os.Chtimes(path, file.AccessTime, file.ModTime)
}
