package nymph

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
)

type pageServer struct {
	cmd *exec.Cmd
	out io.ReadCloser
	err io.ReadCloser
}

func NewPageServer(ckptPath string) (*pageServer, error) {
	var err error

	cmd := exec.Command("criu", "page-server",
		"--port", "7624",
		"--address", "0.0.0.0",
		"--images-dir", ckptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.WithError(err).Error("Did not get the stdout")
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.WithError(err).Error("Did not get the stderr")
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		log.WithError(err).Error("Failed to start page server")
		return nil, err
	}
	return &pageServer{
		out: stdout,
		err: stderr,
		cmd: cmd,
	}, nil
}

func (p *pageServer) Close() {
	if p == nil {
		return
	}

	if slurp, err := ioutil.ReadAll(p.err); err != nil {
		log.WithError(err).Error("Could not read the stdout")
	} else {
		log.Infof("Page server: %s\n", slurp)
	}

	if slurp, err := ioutil.ReadAll(p.out); err != nil {
		log.WithError(err).Error("Could not read the stdout")
	} else {
		log.Infof("Page server: %s\n", slurp)
	}

	if err := p.cmd.Wait(); err != nil {
		log.WithError(err).Error("Page server failed")
	}
}

type Recipient struct {
	nymph      *Nymph
	imageInfo  container.ImageInfoArgs
	rank       container.Rank
	id         string
	seq        int
	generation int
	args       []string

	// Current file data
	Filename string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time

	File    *os.File
	ToWrite int64

	// PageServer
	pageServer *pageServer
}

func NewRecipient(nymph *Nymph) (*Recipient, error) {
	return &Recipient{
		nymph: nymph,
		seq:   4,
	}, nil
}

func (r *Recipient) ImageInfo(args container.ImageInfoArgs, seq *int) error {
	log.WithFields(log.Fields{
		"rank": args.Rank,
		"id":   args.ID,
	}).Debug("Received image info")

	r.imageInfo = args

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) LinkInfo(args container.LinkInfoArgs, seq *int) error {
	log.WithFields(log.Fields{
		"file": args.Filename,
		"link": args.Link,
		"size": args.Size,
		"mode": args.Mode,
		"time": args.ModTime,
	}).Debug("Received link info")

	if r.File != nil {
		log.WithFields(log.Fields{
			"file":     r.File.Name(),
			"to_write": r.ToWrite,
		}).Error("Found unfinished file")
		return fmt.Errorf("Found unfinished file")
	}

	fullpath := path.Join(r.nymph.RootDir, args.Filename)

	dir, _ := path.Split(fullpath)
	if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"dir":      dir,
			"fullpath": fullpath,
			"error":    err,
		}).Error("Failed to create directory")
		return err
	}

	if err := os.Symlink(args.Link, fullpath); err != nil {
		log.WithFields(log.Fields{
			"file":  fullpath,
			"link":  args.Link,
			"error": err,
		}).Debug("Failed to create symlink")
		return fmt.Errorf("Failed to create symlink (%s) -> (%s): %v", args.Link, args.Filename, err)
	}

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) createDir(args container.FileInfoArgs) error {
	dir := path.Join(r.nymph.RootDir, r.Filename)

	if err := os.MkdirAll(dir, args.Mode); err != nil {
		log.WithFields(log.Fields{
			"dir":   dir,
			"error": err,
		}).Error("Failed to create directory")
		return err
	}

	r.File = nil
	r.ToWrite = 0

	return nil
}

func (r *Recipient) createFile(args container.FileInfoArgs) error {
	fullpath := path.Join(r.nymph.RootDir, r.Filename)
	dir, _ := path.Split(fullpath)

	if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"dir":      dir,
			"fullpath": fullpath,
			"error":    err,
		}).Error("Failed to create directory")
		return err
	}

	var err error
	r.File, err = os.OpenFile(fullpath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, r.Mode)
	if err != nil {
		log.WithFields(log.Fields{
			"file":  fullpath,
			"error": err,
		}).Debug("Failed to create file")
		return fmt.Errorf("Failed to create file (%s): %v", r.Filename, err)
	}

	r.ToWrite = r.Size

	return nil
}

func (r *Recipient) FileInfo(args container.FileInfoArgs, seq *int) error {
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

	if r.File != nil {
		log.WithFields(log.Fields{
			"file":     r.File.Name(),
			"to_write": r.ToWrite,
		}).Error("Found unfinished file")
		return fmt.Errorf("Found unfinished file")
	}

	var err error
	if args.Mode.IsDir() {
		err = r.createDir(args)
	} else {
		err = r.createFile(args)
	}

	log.WithFields(log.Fields{
		"error":  err,
		"is-dir": args.Mode.IsDir(),
	}).Debug("Created file")

	*seq = r.seq
	r.seq = r.seq + 1
	return err
}

func (r *Recipient) FileData(args container.FileDataArgs, seq *int) error {
	dataLen := int64(len(args.Data))
	if r.ToWrite < dataLen {
		log.WithFields(log.Fields{
			"size":     dataLen,
			"to_write": r.ToWrite,
		}).Error("Unexpected buffer size")
		return fmt.Errorf("Unexpected buffer size")
	}

	written, err := r.File.Write(args.Data)
	if err != nil {
		return fmt.Errorf("Failed to write the file: %v", err)
	}

	if int64(written) != dataLen {
		return fmt.Errorf("Not all data has been written")
	}

	r.ToWrite = r.ToWrite - dataLen

	if r.ToWrite == 0 {
		r.File.Close()

		r.File = nil
		r.Filename = ""
	}

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) StartPageServer(args container.StartPageServerArgs, seq *int) error {
	log.WithField("checkpoint-path", args.CheckpointPath).Info("Starting page server")

	if err := os.MkdirAll(args.CheckpointPath, os.ModeDir|os.ModePerm); err != nil {
		log.WithError(err).Error("Failed to create checkpoint directory")
		return err
	}

	var err error
	if r.pageServer, err = NewPageServer(args.CheckpointPath); err != nil {
		return err
	}

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) Relaunch(args container.RelaunchArgs, seq *int) error {
	// Load container from checkpoint

	start := time.Now()
	linkName := "/tmp/konk/nymph/checkpoints/93252b437d782ba9d09f77e098b2dc3ef3fb539f026eb2fbc478d9d6f7c45aff/1/parent"
	if fi, err := os.Lstat(linkName); err != nil {
		log.WithError(err).Error("Lstat")
		return err
	} else {
		log.WithField("file-info", fi).Info("Lstot")
	}

	if err := os.Remove(linkName); err != nil {
		log.WithError(err).Info("Remove")
	}
	os.Symlink("../0", linkName)

	cont, err := r.nymph.Containers.Load(r.imageInfo)
	if err != nil {
		log.WithFields(log.Fields{
			"id":   r.id,
			"rank": r.rank,
		}).Error("Loading container has failed")
		return err
	}

	for _, net := range r.nymph.networks {
		if external, ok := net.DeclareExternal(cont.Rank()); ok {
			cont.AddExternal(external)
		}
	}

	if err := cont.Launch(container.Restore, cont.Args(), true); err != nil {
		return err
	}

	for _, net := range r.nymph.networks {
		if err := net.PostRestore(cont); err != nil {
			return err
		}
	}

	status, err := cont.Status()
	if err != nil {
		log.WithError(err).Error("Quering status failed")
		return err
	}

	// Remember old ID
	elapsed := time.Since(start)
	log.WithField("elapsed", elapsed).Info("Checkpoint sent successfully")

	log.WithFields(log.Fields{
		"cont":   cont.ID(),
		"status": status,
	}).Debug("Container has been restored")

	return nil
}

func (r *Recipient) _Close() {
	r.pageServer.Close()
}
