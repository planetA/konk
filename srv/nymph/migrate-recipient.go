package nymph

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	. "github.com/planetA/konk/pkg/nymph"
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

// Receive the checkpoint over a created listener
// func ReceiveListener(listener net.Listener) (container.Container, error) {
// 	migrationServer, err := newServer()
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to create migration server: %v", err)
// 	}
// 	konk.RegisterMigrationServer(grpcServer, migrationServer)

// 	cont := make(chan container.Container, 1)
// 	go func() {
// 		cont <- <-migrationServer.Ready
// 		grpcServer.Stop()
// 	}()

// 	grpcServer.Serve(listener)

// 	return <-cont, nil
// }

type Recipient struct {
	nymph *Nymph
	rank  container.Rank
	id    string
	seq   int

	// Current file data
	Filename string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time
}

func NewRecipient(nymph *Nymph) (*Recipient, error) {
	return &Recipient{
		nymph: nymph,
		seq:   4,
	}, nil
}

func (r *Recipient) Hello(args HelloArgs, seq *int) error {
	log.WithField("say", args.Say).Debug("Received hello")

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) ImageInfo(args ImageInfoArgs, seq *int) error {

	r.rank = args.Rank
	r.id = args.ID

	log.WithFields(log.Fields{
		"rank": args.Rank,
		"id":   args.ID,
	}).Debug("Received image info")

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) FileInfo(args FileInfoArgs, seq *int) error {
	log.WithFields(log.Fields{
		"file": args.Filename,
		"size": args.Size,
		"mode": args.Mode,
		"time": args.ModTime,
	}).Debug("Received file info")

	r.Filename = args.Filename
	r.Size = args.Size
	r.Mode = args.Mode
	r.ModTime = args.ModTime

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) _Close() {

}
