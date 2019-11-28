package nymph

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	"google.golang.org/grpc"
)

type konkMigrationServer struct {
	Rank    container.Rank
	InitPid int

	// Compose the directory where the image is stored
	curFile     *os.File
	curFilePath string
	Ready       chan container.Container
}

func (srv *konkMigrationServer) recvImageInfo(imageInfo *konk.FileData_ImageInfo) (err error) {
	panic("Unimplemented")
	// srv.Rank = container.Rank(imageInfo.ContainerRank)

	// if err := os.MkdirAll(srv.criu.ImageDirPath, os.ModeDir|os.ModePerm); err != nil {
	// 	return fmt.Errorf("Could not create image directory (%s): %v", srv.criu.ImageDirPath, err)
	// }

	return nil
}

func (srv *konkMigrationServer) newFile(fileInfo *konk.FileData_FileInfo) error {
	panic("Unimplemented")
	// if dir := fileInfo.GetDir(); dir != "" {
	// 	srv.curFilePath = fmt.Sprintf("%s/%s", dir, fileInfo.GetFilename())
	// 	if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
	// 		return fmt.Errorf("Could not create image directory (%s): %v", dir, err)
	// 	}
	// } else {
	// 	srv.curFilePath = fmt.Sprintf("%s/%s", srv.criu.ImageDirPath, fileInfo.GetFilename())
	// }

	log.Printf("Creating file: %s\n", srv.curFilePath)

	var err error

	perm := os.FileMode(fileInfo.GetPerm())
	srv.curFile, err = os.OpenFile(srv.curFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		log.Println(srv.curFilePath, err)
		return fmt.Errorf("Failed to create file (%s): %v", srv.curFilePath, err)
	}

	return nil
}

func (srv *konkMigrationServer) recvData(chunk *konk.FileData_FileData) error {
	_, err := srv.curFile.Write(chunk.GetData())
	if err != nil {
		return fmt.Errorf("Failed to write the file: %v", err)
	}

	return nil
}

func (srv *konkMigrationServer) closeFile(fileEnd *konk.FileData_FileEnd) error {
	srv.curFile.Close()

	log.Printf("File received: %v\n", srv.curFilePath)

	srv.curFilePath = ""
	return nil
}

func (srv *konkMigrationServer) launch(launchInfo *konk.FileData_LaunchInfo) (*exec.Cmd, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Printf("Received launch request\n")
	panic("Unimplemented")

	// // XXX: true is very bad style
	// cmd, err := srv.criu.launch(true)
	// if err != nil {
	// 	return nil, fmt.Errorf("Failed to launch criu service: %v", err)
	// }

	// err = srv.criu.sendRestoreRequest()
	// if err != nil {
	// 	return nil, fmt.Errorf("Write to socket failed: %v", err)
	// }

	panic("Not implemented")
}

func isImageInfo(chunk *konk.FileData) bool {
	return chunk.GetImageInfo() != nil
}

func isFileInfo(chunk *konk.FileData) bool {
	return chunk.GetFileInfo() != nil
}

func isFileData(chunk *konk.FileData) bool {
	return chunk.GetFileData() != nil
}

func isFileEnd(chunk *konk.FileData) bool {
	return chunk.GetFileEnd() != nil
}

func isLaunchInfo(chunk *konk.FileData) bool {
	return chunk.GetLaunchInfo() != nil
}

func (srv *konkMigrationServer) Migrate(stream konk.Migration_MigrateServer) error {
	var cmd *exec.Cmd

	defer func() {
		// srv.criu.Close()
		panic("Unimplemented")
		// cont, err := container.NewContainerInitAttach(srv.Rank, srv.InitPid)
		// if err != nil {
		// 	log.Panic("NewContainerInitAttach: %v", err)
		// }
		// srv.Ready <- cont
	}()

loop:
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Migration finished\n")
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read file info from the stream: %v", err)
		}

		switch {
		case isImageInfo(chunk):
			// The first message
			err = srv.recvImageInfo(chunk.GetImageInfo())
		case isFileInfo(chunk):
			// The beginning of a new file
			err = srv.newFile(chunk.GetFileInfo())
		case isFileData(chunk):
			err = srv.recvData(chunk.GetFileData())
		case isFileEnd(chunk):
			err = srv.closeFile(chunk.GetFileEnd())
		case isLaunchInfo(chunk):
			// Got a request to launch the checkpoint
			cmd, err = srv.launch(chunk.GetLaunchInfo())
			if err == nil {
				stream.SendAndClose(&konk.Reply{
					Status: konk.Status_OK,
				})
				break loop
			}
		}

		if err != nil {
			return fmt.Errorf("Failure at processing the next frame: %v", err)
		}
	}

	if cmd == nil {
		return fmt.Errorf("The container was not relaunched")
	}

	return nil
}

func newServer() (*konkMigrationServer, error) {
	s := &konkMigrationServer{
		Ready: make(chan container.Container, 1),
	}

	return s, nil
}

// Receive the checkpoint over a created listener
func ReceiveListener(listener net.Listener) (container.Container, error) {
	grpcServer := grpc.NewServer()
	migrationServer, err := newServer()
	if err != nil {
		return nil, fmt.Errorf("Failed to create migration server: %v", err)
	}
	konk.RegisterMigrationServer(grpcServer, migrationServer)

	cont := make(chan container.Container, 1)
	go func() {
		cont <- <-migrationServer.Ready
		grpcServer.Stop()
	}()

	grpcServer.Serve(listener)

	return <-cont, nil
}
