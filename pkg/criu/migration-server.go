package criu

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	"github.com/planetA/konk/pkg/util"
)

type konkMigrationServer struct {
	// Compose the directory where the image is stored
	criu        *CriuService
	container   *container.Container
	curFile     *os.File
	curFilePath string
	Ready       chan bool
}

func (srv *konkMigrationServer) recvImageInfo(imageInfo *konk.FileData_ImageInfo) (err error) {
	containerId := container.Id(imageInfo.ContainerId)

	srv.container, err = container.NewContainerInit(containerId)
	if err != nil {
		return fmt.Errorf("Failed to create container at the destination: %v", err)
	}

	srv.criu, err = criuFromContainer(srv.container)
	if err != nil {
		return fmt.Errorf("Failed to start criu service: %v", err)
	}

	if err := os.MkdirAll(srv.criu.imageDirPath, os.ModeDir|os.ModePerm); err != nil {
		return fmt.Errorf("Could not create image directory (%s): %v", srv.criu.imageDirPath, err)
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
		srv.curFilePath = fmt.Sprintf("%s/%s", srv.criu.imageDirPath, fileInfo.GetFilename())
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

func (srv *konkMigrationServer) launch(launchInfo *konk.FileData_LaunchInfo) (*exec.Cmd, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Printf("Received launch request\n")

	var err error
	srv.container.Network, err = container.NewNetwork(srv.container.Id, srv.container.Path)
	if err != nil {
		return nil, err
	}

	// XXX: true is very bad style
	cmd, err := srv.criu.launch(srv.container, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to launch criu service: %v", err)
	}

	err = srv.criu.sendRestoreRequest()
	if err != nil {
		return nil, fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := srv.criu.nextEvent()
		log.Println("3", event, err)
		switch event.Type {
		case PreRestore:
			log.Println("@pre-restore")
		case PostRestore:
			log.Println("@post-restore")
		case Success:
			log.Printf("Restore completed: %v", event.Response)
			return cmd, nil
		case Error:
			return nil, fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		srv.criu.respond()
	}

	panic("Should be never reached")
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

	ctx, cancel := util.NewContext()
	go func() {
		select {
		case <-ctx.Done():
			srv.criu.cleanup()
			srv.Ready <- true
		}
	}()
	defer cancel()

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
			log.Printf("Failure at processing the next frame: %v", err)
			os.Exit(1)
			return fmt.Errorf("Failure at processing the next frame: %v", err)
		}
	}

	if cmd == nil {
		return fmt.Errorf("The container was not relaunched")
	}

	cmd.Wait()
	return nil
}

func newServer() (*konkMigrationServer, error) {
	s := &konkMigrationServer{
		Ready: make(chan bool, 1),
	}

	return s, nil
}
