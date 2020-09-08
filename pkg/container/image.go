package container

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

const (
	imageDir = "./images"
)

// Class representing container image
type Image struct {
	RootPath string
	Name     string
	Spec     *specs.Spec
}

func setFileAttributes(fullPath string, header *tar.Header) error {
	mode := header.FileInfo().Mode()
	if mode & os.ModeSymlink == 0 {
		// Make sure that only changeable flags are set
		mode = mode & (os.ModeSticky | os.ModeSetgid | os.ModeSetuid | os.ModePerm)
		if err := os.Chmod(fullPath, mode); err != nil {
			return fmt.Errorf("Failed to set file mode: %v", err)
		}
	}

	if err := os.Lchown(fullPath, header.Uid, header.Gid); err != nil {
		return fmt.Errorf("Failed to set user and group ID for %v: %v", fullPath, err)
	}

	if err := ChtimesFlags(fullPath, header.AccessTime, header.ModTime, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("Failed to set atime and mtime for %v: %v", fullPath, err)
	}

	if len(header.PAXRecords) > 0 {
		log.WithFields(log.Fields{
			"records": len(header.PAXRecords),
			"path":    fullPath,
		}).Debug("Skipping PAX records")
	}

	return nil
}

func createDir(extractDir string, header *tar.Header) error {
	fullPath := path.Join(extractDir, header.Name)

	log.WithFields(log.Fields{
		"path": fullPath,
	}).Trace("Creating directory")
	if err := os.Mkdir(fullPath, header.FileInfo().Mode()); err != nil {
		return fmt.Errorf("Failed to create directory %v: %v", fullPath, err)
	}

	return setFileAttributes(fullPath, header)
}

func writeFile(extractDir string, header *tar.Header, reader io.Reader) error {
	fullPath := path.Join(extractDir, header.Name)
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
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
		"path":     fullPath,
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
		"path":     fullPath,
	}).Trace("Creating a hard link")

	if err := os.Link(linkPath, fullPath); err != nil {
		return fmt.Errorf("Failed to create a hard link %v: %v", fullPath, err)
	}

	return nil
}

// Generate an image name from the container path
func ImageName(imagePath string) string {
	s := sha512.New512_256()
	s.Write([]byte(imagePath))
	return hex.EncodeToString(s.Sum(nil))
}

func unpackImage(extractDir, image string) error {
	imageFile, err := os.Open(image)
	if err != nil {
		return fmt.Errorf("Failed to open container image file %v: %v", image, err)
	}
	defer imageFile.Close()

	gzipFile, err := gzip.NewReader(imageFile)
	if err != nil {
		return fmt.Errorf("File %v is not in gzip format: %v", image, err)
	}

	tarReader := tar.NewReader(gzipFile)
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("Failed reading the archive %v: %v", image, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := createDir(extractDir, header); err != nil {
				return fmt.Errorf("Failed to create directory from image %v: %v", image, err)
			}
		case tar.TypeReg:
			if err := writeFile(extractDir, header, tarReader); err != nil {
				return fmt.Errorf("Failed to write file from image %v: %v", image, err)
			}
		case tar.TypeSymlink:
			if err := createSymLink(extractDir, header); err != nil {
				return fmt.Errorf("Failed to create a symlink from image %v: %v", image, err)
			}
		case tar.TypeLink:
			if err := createHardLink(extractDir, header); err != nil {
				return fmt.Errorf("Failed to create a hardlink from image %v: %v", image, err)
			}
		default:
			return fmt.Errorf("Unsupported file type %v for %v", header.Typeflag, header.Name)
		}
	}

	return nil
}

func readSpec(imageDir string) (*specs.Spec, error) {
	configFilePath := path.Join(imageDir, "config.json")

	log.WithFields(log.Fields{
		"path": configFilePath,
	}).Trace("Reading config file")

	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file %v: %v", configFilePath, err)
	}
	defer configFile.Close()

	var spec specs.Spec

	if err = json.NewDecoder(configFile).Decode(&spec); err != nil {
		return nil, err
	}

	spec.Root.Path = path.Join(imageDir, spec.Root.Path)

	return &spec, nil
}

func NewImage(imagePath string) (*Image, error) {
	name := ImageName(imagePath)
	extractDir := path.Join(imageDir, name)
	// extractDir = "/tmp/4106665f027f0b5280362ac80fb5c92a3af6f066e04f95c89c349434d9d70ad4/"

	log.WithFields(log.Fields{
		"imagePath":  imagePath,
		"name":       name,
		"extractDir": extractDir,
	}).Debug("Getting image name")

	// if err := os.MkdirAll(imageDir, os.ModeDir|os.ModePerm); err != nil {
	// 	return nil, err
	// }

	// if err := unpackImage(extractDir, imagePath); err != nil {
	// 	return nil, err
	// }

	var spec *specs.Spec
	spec, err := readSpec(extractDir)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"path":   imageDir,
		"image":  imagePath,
		"rootfs": spec.Root,
	}).Debug("Converted spec to config")

	return &Image{
		RootPath: extractDir,
		Name:     name,
		Spec:     spec,
	}, nil
}

func (image *Image) Close() {
	// os.RemoveAll(image.RootPath)
}
