package nymph

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

const (
	ChunkSize int = 1 << 21
)

type MigrationDonor struct {
	Checkpoint      container.Checkpoint
	recipientClient *nymph.MigrationClient
	recipient       string
	openFiles       []string
	rootDir         string
}

func NewMigrationDonor(rootDir string, checkpoint container.Checkpoint, recipient string) (*MigrationDonor, error) {
	client, err := nymph.NewMigrationClient(recipient)
	if err != nil {
		log.WithError(err).Error("Client creation failed")
		return nil, fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}

	return &MigrationDonor{
		Checkpoint:      checkpoint,
		recipientClient: client,
		recipient:       recipient,
		rootDir:         rootDir,
	}, nil
}

func (m *MigrationDonor) sendState() error {
	err := m.recipientClient.ImageInfo(m.Checkpoint.ImageInfo())
	if err != nil {
		return err
	}

	stateFile := m.Checkpoint.StatePath()

	if err := m.SendFile(stateFile); err != nil {
		return fmt.Errorf("Failed to transfer the file %s: %v", stateFile, err)
	}

	log.WithField("name", stateFile).Debug("Sent a file")
	return nil
}

func (migration *MigrationDonor) sendImage() error {
	checkpointDir, err := os.Open(migration.Checkpoint.PathAbs())
	if err != nil {
		log.WithFields(log.Fields{
			"dir":   migration.Checkpoint.PathAbs(),
			"error": err,
		}).Error("Failed to open checkpoint dir")
		return fmt.Errorf("Failed to open checkpoint dir: %v", err)
	}

	files, err := checkpointDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of checkpoint directory: %v", err)
	}

	for _, file := range files {
		filename := path.Join(migration.Checkpoint.Path(), file.Name())
		err := migration.SendFile(filename)
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", filename, err)
		}

		log.WithField("name", filename).Debug("Sent a file")
	}

	return nil
}

func (migration *MigrationDonor) sendTmpFiles(cont *container.Container) error {
	rootfs := cont.Config().Rootfs
	tmpPath := fmt.Sprintf("%s/tmp", rootfs)

	if !strings.HasPrefix(tmpPath, migration.rootDir) {
		return fmt.Errorf("Path mismatch")
	}

	err := filepath.Walk(tmpPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			filePath := path[len(migration.rootDir):]

			if !(info.IsDir() || info.Mode().IsRegular()) {
				log.WithFields(log.Fields{
					"file-path": filePath,
					"mode":      info.Mode(),
				}).Debug("Skipping file")
				return nil
			}

			return migration.SendFile(filePath)
		})

	if err != nil {
		log.WithError(err).Error("Failed to get files")
	}
	return err
}

// Send file path relative to container directory root.
func (migration *MigrationDonor) SendFile(filepath string) error {
	fullpath := path.Join(migration.rootDir, filepath)

	fileInfo, err := os.Lstat(fullpath)
	if err != nil {
		return fmt.Errorf("Failed to get file state: %v", err)
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(fullpath)
		if err != nil {
			return err
		}
		return migration.recipientClient.LinkInfo(filepath, fileInfo, link)
	}

	err = migration.recipientClient.FileInfo(filepath, fileInfo)
	if err != nil {
		return fmt.Errorf("Failed to send file info %s: %v", filepath, err)
	}

	log.WithFields(log.Fields{
		"file":   filepath,
		"is-dir": fileInfo.IsDir(),
	}).Debug("Sent file info")

	if fileInfo.IsDir() {
		return nil
	}


	file, err := os.Open(fullpath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v", err)
	}
	defer file.Close()

	buf := make([]byte, ChunkSize)

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Error while reading file: %v", err)
		}

		err = migration.recipientClient.FileData(buf[:n])
		if err != nil {
			return fmt.Errorf("Error while sending data: %v", err)
		}
	}

	return nil
}

func (migration *MigrationDonor) StartPageServer(checkpointPath string) error {

	err := migration.recipientClient.StartPageServer(checkpointPath)
	if err != nil {
		log.WithError(err).Debug("Requested launch failed")
		return fmt.Errorf("Failed to send launch request: %v", err)
	}

	return nil
}

func (migration *MigrationDonor) Relaunch() error {

	err := migration.recipientClient.Relaunch()
	if err != nil {
		log.WithError(err).Debug("Requested launch failed")
		return fmt.Errorf("Failed to send launch request: %v", err)
	}

	return nil
}

func (migration *MigrationDonor) SendCheckpoint(cont *container.Container) error {
	if err := migration.sendState(); err != nil {
		return err
	}

	if err := migration.sendImage(); err != nil {
		return err
	}

	if cont != nil {
		if err := migration.sendTmpFiles(cont); err != nil {
			return err
		}
	}

	return nil
}

func (migration *MigrationDonor) Close() {
	migration.recipientClient.Close()
}
