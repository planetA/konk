package criu

import (
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/konk"
)

type MigrationClient struct {
	konk.Migration_MigrateClient
	LocalDir string
}

func (migration *MigrationClient) SendImageInfo(containerId int) error {
	log.Printf("Sending image info")

	err := migration.Send(&konk.FileData{
		ImageInfo: &konk.FileData_ImageInfo{
			ImagePath:   migration.LocalDir,
			ContainerId: int32(containerId),
		},
	})

	if err != nil {
		return fmt.Errorf("Failed to send image info: %v", err)
	}

	return nil
}

func (migration *MigrationClient) SendFile(file string) error {
	return migration.SendFileDir(file, "")
}

// send file by its full path. If a path is relative, the file is looked up in
// the local image directory.
func (migration *MigrationClient) SendFileDir(path string, dir string) error {
	localDir := dir
	if len(localDir) == 0 {
		// If the path is relative the file is looked up in the image directory
		localDir = migration.LocalDir
	}
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
			Dir: dir,
			Perm: int32(fileInfo.Mode().Perm()),
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

func (migration *MigrationClient) Launch() error {
	log.Printf("Requested launch")

	err := migration.Send(&konk.FileData{
		LaunchInfo: &konk.FileData_LaunchInfo{
			ContainerId: -1,
		},
	})

	if err != nil {
		return fmt.Errorf("Failed to send launch request: %v", err)
	}

	return nil
}

func (migration *MigrationClient) Close() {
	migration.CloseSend()

	reply, err := migration.CloseAndRecv()
	if err != nil {
		log.Printf("Error while closing the stream: %v\n", err)
	}
	if reply.GetStatus() != konk.Status_OK {
		log.Printf("File transfer failed: %s\n", reply.GetStatus())
	}

}

func newMigrationClient(conn *grpc.ClientConn, localDir string) (*MigrationClient, error) {
	ctx := context.Background()
	client := konk.NewMigrationClient(conn)

	stream, err := client.Migrate(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to create stream: %v", err)
	}

	return &MigrationClient{
		Migration_MigrateClient: stream,
		LocalDir:                localDir,
	}, nil
}
