package nymph

import (
	"fmt"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	// "golang.org/x/net/context"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
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
}

func NewMigrationDonor(container *container.Container, recipient string) (*MigrationDonor, error) {
	client, err := nymph.NewMigrationClient(recipient)
	if err != nil {
		log.WithError(err).Error("Client creation failed")
		return nil, fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}

	return &MigrationDonor{
		Container:       container,
		recipientClient: client,
		recipient:       recipient,
	}, nil
}

func (migration *MigrationDonor) Send(b interface{}) error {
	return nil
}

func (migration *MigrationDonor) sendState() error {
	state, err := migration.Container.State()
	if err != nil {
		return err
	}

	if err := migration.recipientClient.ImageInfo(migration.Container.Rank(), state.ID); err != nil {
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
	imageDir, err := os.Open(migration.Container.CheckpointPath())
	if err != nil {
		log.WithFields(log.Fields{
			"dir":   migration.Container.CheckpointPath(),
			"error": err,
		}).Error("Failed to open checkpoint dir")
		return fmt.Errorf("Failed to open checkpoint dir: err", err)
	}

	files, err := imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	for _, file := range files {
		fullpath := path.Join(migration.Container.CheckpointPath(), file.Name())
		err := migration.SendFile(fullpath)
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", file.Name(), err)
		}

		log.WithField("name", file.Name()).Debug("Sent a file")
	}

	return nil
}

// Send file path relative to container directory root.
func (migration *MigrationDonor) SendFile(filepath string) error {
	file, err := os.Open(filepath)
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

	return nil

	buf := make([]byte, ChunkSize)

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Error while reading file: %v", err)
		}

		err = migration.Send(&konk.FileData{
			FileData: &konk.FileData_FileData{
				Data: buf[:n],
			},
		})
		if err != nil {
			return fmt.Errorf("Error while sending data: %v", err)
		}
	}

	// Notify that the file transfer has ended
	err = migration.Send(&konk.FileData{
		FileEnd: &konk.FileData_FileEnd{
			EndMarker: true,
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to send end marker (%s): %v", file.Name(), err)
	}

	return nil
}

func (migration *MigrationDonor) Launch() error {
	log.Printf("Requested launch")

	err := migration.Send(&konk.FileData{
		LaunchInfo: &konk.FileData_LaunchInfo{
			ContainerRank: -1,
		},
	})

	if err != nil {
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
