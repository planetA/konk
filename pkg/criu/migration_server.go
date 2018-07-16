package criu

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/planetA/konk/pkg/konk"
)

type konkMigrationServer struct {
	// Compose the directory where the image is stored
	imageDir string
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
	return nil
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
		case isFileInfo(chunk):
			// The beginning of a new file
			err = srv.recvFile(stream, chunk.GetFileInfo())
		case isLaunchInfo(chunk):
			// Got a request to launch the checkpoint
			err = srv.launch(chunk.GetLaunchInfo())
		}

		if err != nil {
			return fmt.Errorf("Failure at processing the next frame: %v", err)
		}
	}

	return stream.SendAndClose(&konk.Reply{
		Status: konk.Status_OK,
	})
}

func newServer() (*konkMigrationServer, error) {
	rand.Seed(time.Now().UTC().UnixNano())
	imageDir := fmt.Sprintf("%s/criu.image.%v", os.TempDir(), rand.Intn(32767))
	s := &konkMigrationServer{
		imageDir: imageDir,
	}

	// Create the directory where to store the image
	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		if err = os.Mkdir(imageDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("Failed create a directory on the recipient: %v", err)
		}
	}
	return s, nil
}
