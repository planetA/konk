package criu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
)

type MigrationClient struct {
	konk.Migration_MigrateClient
	LocalDir   string
	Container  *container.Container
	ServerConn *grpc.ClientConn // Connection to the migration server
	Criu       *CriuService
}

func (migration *MigrationClient) SendImageInfo(containerId container.Id) error {
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

func (migration *MigrationClient) sendImage(imageDir *os.File) error {
	files, err := imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	if err = migration.SendImageInfo(migration.Container.Id); err != nil {
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

func (migration *MigrationClient) sendOpenFiles(pid int, prefix string) error {
	linksDirPath := fmt.Sprintf("/proc/%d/map_files/", pid)

	files, err := ioutil.ReadDir(linksDirPath)
	if err != nil {
		return fmt.Errorf("Failed to open directory %s: %v", linksDirPath, err)
	}

	prefixLen := len(prefix)
	for _, fdName := range files {
		fdPath := fmt.Sprintf("%s/%s", linksDirPath, fdName.Name())
		filePath, err := os.Readlink(fdPath)
		if filePath[:prefixLen] != prefix {
			continue
		}

		err = migration.SendFileDir(filepath.Base(filePath), filepath.Dir(filePath))
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", filePath, err)
		}

		log.Printf("Sent a file: %v", filePath)
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

func (migration *MigrationClient) sendCheckpoint() error {
	if err := migration.sendImage(migration.Criu.imageDir); err != nil {
		return err
	}

	if err := migration.sendOpenFiles(migration.Criu.targetPid, "/tmp"); err != nil {
		return err
	}

	return nil
}

func (migration *MigrationClient) Run(ctx context.Context) error {
	// Launch actual CRIU process
	// XXX: false is very bad style
	if _, err := migration.Criu.launch(migration.Container, false); err != nil {
		return fmt.Errorf("Failed to launch criu service: %v", err)
	}

	err := migration.Criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := migration.Criu.nextEvent()
		switch event.Type {
		case PreDump:
			log.Printf("@pre-dump")
		case PostDump:
			log.Printf("@pre-move")

			err = migration.sendCheckpoint()
			if err != nil {
				return fmt.Errorf("Failed to save a checkpoint: %v", err)
			}

			log.Printf("XXX: Need to ensure that container does not exists")

			if err = migration.Launch(); err != nil {
				return err
			}

			log.Printf("@post-move")
		case Success:
			log.Printf("Dump completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		migration.Criu.respond()
	}
}

func (migration *MigrationClient) Close() {
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

	migration.ServerConn.Close()
	migration.Criu.cleanup()
	migration.Container.Delete()
}

func newMigrationClient(ctx context.Context, recipient string, pid int) (*MigrationClient, error) {
	log.Println("Connecting to", recipient)

	conn, err := grpc.Dial(recipient, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}

	client := konk.NewMigrationClient(conn)

	// Create to transfer the data over
	stream, err := client.Migrate(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to create stream: %v", err)
	}

	// Create Criu object that is configure to start the real service
	criu, err := criuFromPid(pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}

	// Get handlers to container host and guest namespaces
	cont, err := container.ContainerAttachPid(pid)
	if err != nil {
		return nil, fmt.Errorf("Could not attach to a container: %v", err)
	}

	return &MigrationClient{
		Migration_MigrateClient: stream,
		LocalDir:                criu.imageDirPath,
		ServerConn:              conn,
		Container:               cont,
		Criu:                    criu,
	}, nil
}
