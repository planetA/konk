package nymph

import (
	"fmt"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	// "golang.org/x/net/context"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

const (
	ChunkSize int = 1 << 21
)

type MigrationDonor struct {
	Container       *container.Container
	recipientClient *nymph.MigrationClient
	recipient       string
	openFiles       []string
	rootDir         string
}

func NewMigrationDonor(rootDir string, container *container.Container, recipient string) (*MigrationDonor, error) {
	client, err := nymph.NewMigrationClient(recipient)
	if err != nil {
		log.WithError(err).Error("Client creation failed")
		return nil, fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}

	return &MigrationDonor{
		Container:       container,
		recipientClient: client,
		recipient:       recipient,
		rootDir:         rootDir,
	}, nil
}

func (migration *MigrationDonor) sendState() error {
	state, err := migration.Container.State()
	if err != nil {
		return err
	}

	if err := migration.recipientClient.ImageInfo(migration.Container.Rank(), state.ID, migration.Container.Args()); err != nil {
		return err
	}

	stateFile := migration.Container.StatePath()

	if err := migration.SendFile(stateFile); err != nil {
		return fmt.Errorf("Failed to transfer the file %s: %v", stateFile, err)
	}

	log.WithField("name", stateFile).Debug("Sent a file")
	return nil
}

func (migration *MigrationDonor) sendImage() error {
	checkpointDir, err := os.Open(migration.Container.CheckpointPathAbs())
	if err != nil {
		log.WithFields(log.Fields{
			"dir":   migration.Container.CheckpointPathAbs(),
			"error": err,
		}).Error("Failed to open checkpoint dir")
		return fmt.Errorf("Failed to open checkpoint dir: %v", err)
	}

	files, err := checkpointDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of checkpoint directory: %v", err)
	}

	for _, file := range files {
		filename := path.Join(migration.Container.CheckpointPath(), file.Name())
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
	// reply, err := migration.CloseAndRecv()
	// if err != nil {
	// 	log.Printf("Error while closing the stream: %v\n", err)
	// 	log.Println("XXX: This should not happen. But I don't know how to fix it for now")
	// 	log.Println("The reason is connected to creating a new container on another" +
	// 		" machine and this somehow distorts the network connection")
	// }
	// if reply.GetStatus() != konk.Status_OK {
	// 	log.Printf("File transfer failed: %s\n", reply.GetStatus())
	// }

	migration.recipientClient.Close()
	// migration.Container.Close()
}
