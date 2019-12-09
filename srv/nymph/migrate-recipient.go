package nymph

import (
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	. "github.com/planetA/konk/pkg/nymph"
)

type Recipient struct {
	nymph *Nymph
	rank  container.Rank
	id    string
	seq   int
	args  []string

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

func (r *Recipient) ImageInfo(args ImageInfoArgs, seq *int) error {

	r.rank = args.Rank
	r.id = args.ID
	r.args = args.Args

	log.WithFields(log.Fields{
		"rank": args.Rank,
		"id":   args.ID,
	}).Debug("Received image info")

	containerPath := path.Join(r.nymph.Containers.PathAbs(), args.ID)

	if err := os.MkdirAll(containerPath, os.ModeDir|os.ModePerm); err != nil {
		return err
	}

	checkpointPath := path.Join(r.nymph.Containers.CheckpointsPathAbs(), args.ID)
	if err := os.MkdirAll(checkpointPath, os.ModeDir|os.ModePerm); err != nil {
		return err
	}

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

	if r.File != nil {
		log.WithFields(log.Fields{
			"file":     r.File.Name(),
			"to_write": r.ToWrite,
		}).Error("Found unfinished file")
		return fmt.Errorf("Found unfinished file")
	}

	var err error
	fullpath := path.Join(r.nymph.RootDir, r.Filename)
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

func (r *Recipient) FileData(args FileDataArgs, seq *int) error {
	dataLen := int64(len(args.Data))
	log.WithFields(log.Fields{
		"size":     dataLen,
		"to_write": r.ToWrite,
	}).Trace("Sending data chunk")

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
		log.Trace("Closing file")
		r.File.Close()

		r.File = nil
		r.Filename = ""
	}

	*seq = r.seq
	r.seq = r.seq + 1
	return nil
}

func (r *Recipient) Relaunch(args RelaunchArgs, seq *int) error {
	// Load container from checkpoint

	cont, err := r.nymph.Containers.Load(r.rank, r.id, r.args)
	if err != nil {
		log.WithFields(log.Fields{
			"id":   r.id,
			"rank": r.rank,
		}).Error("Loading container has failed")
		return err
	}

	cont.AddExternal(r.nymph.network.DeclareExternal(cont.Rank()))

	if err := cont.Launch(container.Restore); err != nil {
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
