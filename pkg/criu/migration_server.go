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
	containerId int
}

func (srv *konkMigrationServer) recvImageInfo(imageInfo *konk.FileData_ImageInfo) error {
	srv.containerId = int(imageInfo.ContainerId)
	srv.imageDir = imageInfo.ImagePath

	log.Printf("Local directory: %v", srv.imageDir)

	if err := os.MkdirAll(srv.imageDir, os.ModeDir|os.ModePerm); err != nil {
		return fmt.Errorf("Could not create image directory (%s): %v", srv.imageDir, err)
	}

	return nil
}

func (srv *konkMigrationServer) recvFile(stream konk.Migration_MigrateServer, fileInfo *konk.FileData_FileInfo) error {
	filePath := fmt.Sprintf("%s/%s", srv.imageDir, fileInfo.GetFilename())

	log.Printf("Creating file: %s\n", filePath)

	file, err := os.Create(filePath)
	if err != nil {
		log.Println(filePath, err)
		return fmt.Errorf("Failed to create file (%s): %v", filePath, err)
	}
	defer file.Close()

	for {
		chunk, err := stream.Recv()
		if err != nil {
			return err
		}

		if chunk.GetEndMarker() {
			return nil
		}

		_, err = file.Write(chunk.GetData())
		if err != nil {
			return fmt.Errorf("Failed to write the file: %v", err)
		}
	}

	log.Printf("File received: %v\n", filePath)

	return nil
}

func (srv *konkMigrationServer) launch(launchInfo *konk.FileData_LaunchInfo) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := container.Create(srv.containerId)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}
	defer container.Delete(srv.containerId)

	criu, err := criuFromContainer(srv.containerId, srv.imageDir)
	if err != nil {
		return fmt.Errorf("Failed to start criu service: %v", err)
	}
	defer criu.cleanupService()

	log.Printf("Received launch request\n")

	err = criu.sendRestoreRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := criu.nextEvent()
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

		criu.respond()
	}

	return nil
}

func isImageInfo(chunk *konk.FileData) bool {
	return chunk.GetImageInfo() != nil
}

func isFileInfo(chunk *konk.FileData) bool {
	return chunk.GetFileInfo() != nil
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
			err = srv.recvFile(stream, chunk.GetFileInfo())
		case isLaunchInfo(chunk):
			// Got a request to launch the checkpoint
			err = srv.launch(chunk.GetLaunchInfo())
		}

		if err != nil {
			log.Printf("Failure at processing the next frame: %v", err)
			os.Exit(1)
			return fmt.Errorf("Failure at processing the next frame: %v", err)
		}
	}

	return stream.SendAndClose(&konk.Reply{
		Status: konk.Status_OK,
	})
}

func newServer() (*konkMigrationServer, error) {
	s := &konkMigrationServer{}

	return s, nil
}
