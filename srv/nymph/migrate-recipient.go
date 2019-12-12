package nymph

import (
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
)

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

	r.File, err = os.OpenFile(fullpath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, r.Mode)
	if err != nil {
		log.WithFields(log.Fields{
			"file":  fullpath,
			"error": err,
		}).Debug("Failed to create file")
		return fmt.Errorf("Failed to create file (%s): %v", r.Filename, err)
	}

	r.ToWrite = r.Size

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
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

func (r *Recipient) Relaunch(args container.RelaunchArgs, seq *int) error {
	// Load container from checkpoint

	cont, err := r.nymph.Containers.Load(r.imageInfo)
	if err != nil {
		log.WithFields(log.Fields{
			"id":   r.id,
			"rank": r.rank,
		}).Error("Loading container has failed")
		return err
	}

	cont.AddExternal(r.nymph.network.DeclareExternal(cont.Rank()))

	if err := cont.Launch(container.Restore, cont.Args()); err != nil {
		return err
	}

	if err := r.nymph.network.PostRestore(cont); err != nil {
		return err
	}

	status, err := cont.Status()
	if err != nil {
		log.WithError(err).Error("Quering status failed")
		return err
	}

	// Remember old ID

	log.WithFields(log.Fields{
		"cont":   cont.ID(),
		"status": status,
	}).Debug("Container has been restored")

	return nil
}

func (r *Recipient) _Close() {

}
