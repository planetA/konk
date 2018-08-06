package criu

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
)

type konkMigrationServer struct {
	// Compose the directory where the image is stored
	imageDir    string
	criu        *CriuService
	container   *container.Container
	containerId int
	curFile     *os.File
	curFilePath string
	Ready       chan bool
}

func (srv *konkMigrationServer) recvImageInfo(imageInfo *konk.FileData_ImageInfo) error {
	srv.containerId = int(imageInfo.ContainerId)
	srv.imageDir = imageInfo.ImagePath

	log.Printf("Local directory: %v", srv.imageDir)

	if err := os.MkdirAll(srv.imageDir, os.ModeDir|os.ModePerm); err != nil {
		return fmt.Errorf("Could not create image directory (%s): %v", srv.imageDir, err)
	}

	var err error
	srv.criu, err = criuFromContainer(srv.containerId, srv.imageDir)
	if err != nil {
		return fmt.Errorf("Failed to start criu service: %v", err)
	}

	return nil
}

func (srv *konkMigrationServer) newFile(fileInfo *konk.FileData_FileInfo) error {
	if dir := fileInfo.GetDir(); dir != "" {
		srv.curFilePath = fmt.Sprintf("%s/%s", dir, fileInfo.GetFilename())
		if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
			return fmt.Errorf("Could not create image directory (%s): %v", dir, err)
		}
	} else {
		srv.curFilePath = fmt.Sprintf("%s/%s", srv.imageDir, fileInfo.GetFilename())
	}

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

func (srv *konkMigrationServer) launch(launchInfo *konk.FileData_LaunchInfo) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var err error
	srv.container, err = container.CreateContainer(srv.containerId)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}

	log.Printf("Received launch request\n")

	if err = srv.criu.launch(srv.container); err != nil {
		return fmt.Errorf("Failed to launch criu service: %v", err)
	}

	err = srv.criu.sendRestoreRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := srv.criu.nextEvent()
		switch event.Type {
		case PreRestore:
			log.Println("@pre-restore")
		case PostRestore:
			log.Println("@post-restore")
		case Success:
			log.Printf("Restore completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		srv.criu.respond()
	}

	return nil
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

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Transfer finished\n")
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
			err = srv.launch(chunk.GetLaunchInfo())
			stream.SendAndClose(&konk.Reply{
				Status: konk.Status_OK,
			})
			srv.criu.cleanup()
		}

		if err != nil {
			log.Printf("Failure at processing the next frame: %v", err)
			os.Exit(1)
			return fmt.Errorf("Failure at processing the next frame: %v", err)
		}
	}

	srv.Ready <- true
	return nil
}

func newServer() (*konkMigrationServer, error) {
	s := &konkMigrationServer{
		Ready: make(chan bool, 1),
	}

	return s, nil
}
