package nymph

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	"github.com/planetA/konk/pkg/util"
)

const (
	ChunkSize int = 1 << 21
)

type MigrationDonor struct {
	konk.Migration_MigrateClient
	Container     container.Container
	RecipientConn *grpc.ClientConn // Connection to the migration server
	openFiles     []string
}

func (migration *MigrationDonor) SendImageInfo(containerRank container.Rank) error {
	log.Printf("Sending image info")

	err := migration.Send(&konk.FileData{
		ImageInfo: &konk.FileData_ImageInfo{
			ContainerRank: int32(containerRank),
		},
	})

	if err != nil {
		return fmt.Errorf("Failed to send image info: %v", err)
	}

	return nil
}

func (migration *MigrationDonor) sendImage(imageDir *os.File) error {
	files, err := imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	if err = migration.SendImageInfo(migration.Container.Rank()); err != nil {
		return err
	}

	for _, file := range files {
		err := migration.SendFile(file.Name())
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", file.Name(), err)
		}

		log.Printf("Sent a file: %v", file.Name())
	}

	return nil
}

func (migration *MigrationDonor) SendFile(file string) error {
	return migration.SendFileDir(file, "")
}

// send file by its full path. If a path is relative, the file is looked up in
// the local image directory.
func (migration *MigrationDonor) SendFileDir(path string, dir string) error {
	panic("Unimplemented")
	localDir := dir
	// if len(localDir) == 0 {
	// 	// If the path is relative the file is looked up in the image directory
	// 	localDir = migration.Criu.ImageDirPath
	// }
	localPath := fmt.Sprintf("%s/%s", localDir, path)

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get file state: %v", err)
	}

	err = migration.Send(&konk.FileData{
		FileInfo: &konk.FileData_FileInfo{
			Filename: path,
			Dir:      dir,
			Perm:     int32(fileInfo.Mode().Perm()),
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to send file info %s: %v", path, err)
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
		return fmt.Errorf("Failed to send end marker (%s): %v", path, err)
	}

	return nil
}

func (migration *MigrationDonor) sendOpenFiles() error {
	log.Println("Open files: ", migration.openFiles)
	for _, filePath := range migration.openFiles {
		err := migration.SendFileDir(filepath.Base(filePath), filepath.Dir(filePath))
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", filePath, err)
		}

		log.Printf("Sent a file: %v", filePath)
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

func (migration *MigrationDonor) sendCheckpoint() error {
	panic("Unimplemented")
	// if err := migration.sendImage(migration.Criu.imageDir); err != nil {
	// 	return err
	// }

	if err := migration.sendOpenFiles(); err != nil {
		return err
	}

	return nil
}

func (migration *MigrationDonor) Close() {
	reply, err := migration.CloseAndRecv()
	if err != nil {
		log.Printf("Error while closing the stream: %v\n", err)
		log.Println("XXX: This should not happen. But I don't know how to fix it for now")
		log.Println("The reason is connected to creating a new container on another" +
			" machine and this somehow distorts the network connection")
	}
	if reply.GetStatus() != konk.Status_OK {
		log.Printf("File transfer failed: %s\n", reply.GetStatus())
	}

	migration.RecipientConn.Close()
	// migration.Container.Close()
}

func newMigrationDonor(ctx context.Context, recipientAddr string, cont container.Container) (*MigrationDonor, error) {
	log.Println("Connecting to", recipientAddr)

	// Connect to recipient
	conn, err := grpc.Dial(recipientAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}

	recipient := konk.NewMigrationClient(conn)

	// Create a stream to transfer the data over
	stream, err := recipient.Migrate(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to create stream: %v", err)
	}

	return &MigrationDonor{
		Migration_MigrateClient: stream,
		RecipientConn:           conn,
		Container:               cont,
	}, nil
}

func Migrate(cont container.Container, recipient string) error {
	ctx, _ := util.NewContext()
	migration, err := newMigrationDonor(ctx, recipient, cont)
	if err != nil {
		return err
	}
	go func() {
		select {
		case <-ctx.Done():
			migration.Close()
		}
	}()
	defer func() {
		migration.Close()
	}()

	err = migration.sendCheckpoint()
	if err != nil {
		return fmt.Errorf("Failed to save a checkpoint: %v", err)
	}

	log.Printf("XXX: Need to ensure that container does not exists locally")

	if err = migration.Launch(); err != nil {
		return fmt.Errorf("Migration failed launch a container: %v", err)
	}

	return nil
}
