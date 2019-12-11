package nymph

import (
	"fmt"
	"io"
	"os"
	"path"

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

// Send file path relative to container directory root.
func (migration *MigrationDonor) SendFile(filepath string) error {
	fullpath := path.Join(migration.rootDir, filepath)
	file, err := os.Open(fullpath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get file state: %v", err)
	}

	err = migration.recipientClient.FileInfo(filepath, fileInfo)
	if err != nil {
		return fmt.Errorf("Failed to send file info %s: %v", file.Name(), err)
	}

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

func (migration *MigrationDonor) Relaunch() error {

	err := migration.recipientClient.Relaunch()
	if err != nil {
		log.WithError(err).Debug("Requested launch failed")
		return fmt.Errorf("Failed to send launch request: %v", err)
	}

	return nil
}

func (migration *MigrationDonor) SendCheckpoint() error {
	if err := migration.sendState(); err != nil {
		return err
	}

	if err := migration.sendImage(); err != nil {
		return err
	}

	return nil
}

func (migration *MigrationDonor) Close() {
	migration.recipientClient.Close()
}
