package container

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"path"
	"os"
	"io"

	log "github.com/sirupsen/logrus"

	"golang.org/x/sys/unix"
)

// Class representing container image
type Image struct {
	path string
}

// Convert permission and mode from tar to os.FileMode. Potentially need
// sanitising the input.
func tarToOsMode(header *tar.Header) os.FileMode {
	return os.FileMode(header.Mode)
}

func setFileAttributes(fullPath string, header *tar.Header) error {
	if err := os.Lchown(fullPath, header.Uid, header.Gid); err != nil {
		return fmt.Errorf("Failed to set user and group ID for %v: %v", fullPath, err)
	}

	if err := ChtimesFlags(fullPath, header.AccessTime, header.ModTime, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("Failed to set atime and mtime for %v: %v", fullPath, err)
	}

	if len(header.PAXRecords) > 0 {
		log.WithFields(log.Fields{
			"records": len(header.PAXRecords),
			"path": fullPath,
		}).Debug("Skipping PAX records")
	}

	return nil
}

func createDir(extractDir string, header *tar.Header) error {
	fullPath := path.Join(extractDir, header.Name)

	log.WithFields(log.Fields{
		"path": fullPath,
	}).Trace("Creating directory")
	if err := os.Mkdir(fullPath, tarToOsMode(header)); err != nil {
		return fmt.Errorf("Failed to create directory %v: %v", fullPath, err)
	}

	return setFileAttributes(fullPath, header)
}

func writeFile(extractDir string, header *tar.Header, reader io.Reader) error {
	fullPath := path.Join(extractDir, header.Name)
	file, err := os.OpenFile(fullPath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, tarToOsMode(header))
	if err != nil {
		return fmt.Errorf("Failed to create file %v: %v", fullPath, err)
	}
	defer file.Close()

	log.WithFields(log.Fields{
		"path": fullPath,
	}).Trace("Creating file")

	written, err := io.CopyN(file, reader, header.Size)
	if err != nil || written != header.Size {
		return fmt.Errorf("Wrote %v bytes out of %v. Failed: %v", written, header.Size, err)
	}

	return setFileAttributes(fullPath, header)
}

func createSymLink(extractDir string, header *tar.Header) error {
	fullPath := path.Join(extractDir, header.Name)

	log.WithFields(log.Fields{
		"linkname": header.Linkname,
		"path": fullPath,
	}).Trace("Creating a symlink")

	if err := os.Symlink(header.Linkname, fullPath); err != nil {
		return fmt.Errorf("Failed to create a symbolic link %v: %v", fullPath, err)
	}

	return setFileAttributes(fullPath, header)
}

func createHardLink(extractDir string, header *tar.Header) error {
	fullPath := path.Join(extractDir, header.Name)
	linkPath := path.Join(extractDir, header.Linkname)

	log.WithFields(log.Fields{
		"linkname": linkPath,
		"path": fullPath,
	}).Trace("Creating a hard link")

	if err := os.Link(linkPath, fullPath); err != nil {
		return fmt.Errorf("Failed to create a hard link %v: %v", fullPath, err)
	}

	return nil
}

func NewImage(imageDir string, image string) (*Image, error) {
	log.WithFields(log.Fields{
		"path": imageDir,
		"image": image,
	}).Debug("Creating an image")

	imageFile, err := os.Open(image)
	if err != nil {
		return nil, fmt.Errorf("Failed to open container image file %v: %v", image, err)
	}
	defer imageFile.Close()

	gzipFile, err := gzip.NewReader(imageFile)
	if err != nil {
		return nil, fmt.Errorf("File %v is not in gzip format: %v", image, err)
	}

	tarReader := tar.NewReader(gzipFile)
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("Failed reading the archive %v: %v", image, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := createDir(imageDir, header); err != nil {
				return nil, fmt.Errorf("Failed to create directory from image %v: %v", image, err)
			}
		case tar.TypeReg:
			if err := writeFile(imageDir, header, tarReader); err != nil {
				return nil, fmt.Errorf("Failed to write file from image %v: %v", image, err)
			}
		case tar.TypeSymlink:
			if err := createSymLink(imageDir, header); err != nil {
				return nil, fmt.Errorf("Failed to create a symlink from image %v: %v", image, err)
			}
		case tar.TypeLink:
			if err := createHardLink(imageDir, header); err != nil {
				return nil, fmt.Errorf("Failed to create a hardlink from image %v: %v", image, err)
			}
		default:
			return nil, fmt.Errorf("Unsupported file type %v for %v", header.Typeflag, header.Name)
		}
	}

	return nil, nil
}

func (image *Image) NewContainer(id Id) (*Container, error) {
	return nil, nil
}

func (image *Image) Close() {
	panic("Unimplemented")
}
