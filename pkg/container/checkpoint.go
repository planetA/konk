package container

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/opencontainers/runc/libcontainer"
	log "github.com/sirupsen/logrus"
)

type Checkpoint interface {
	// Id of a checkpoint
	ID() string

	// Id of a container owning a checkpoint
	ContainerID() string

	Rank() Rank

	// Dump process into checkpoint
	Dump(preDump bool) error

	// Restore process from checkpoint
	Restore(process *libcontainer.Process) error

	// Path to the current state file
	StatePath() string

	// Path to checkpoint directory starting from nymph root
	Path() string

	// Absolute path to checkpoint directory
	PathAbs() string

	// Args used to launch the container
	Args() []string
}

type checkpoint struct {
	id        string
	container *Container
	last      Checkpoint
}

func (c *Container) NewCheckpoint() (Checkpoint, error) {
	checkpoint := &checkpoint{
		id:        fmt.Sprintf("%v", c.nextCheckpointId),
		container: c,
		last:      c.latestCheckpoint(),
	}

	if err := os.MkdirAll(checkpoint.PathAbs(), os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"dir":   checkpoint.PathAbs(),
		}).Error("Failed to create directory")
		return nil, err
	}

	c.checkpoints = append(c.checkpoints, checkpoint)
	c.nextCheckpointId = c.nextCheckpointId + 1

	return checkpoint, nil
}

func (c *Container) LoadCheckpoints() error {
	files, err := ioutil.ReadDir(c.CheckpointsPath())
	if err != nil {
		log.WithError(err).WithField("dir", c.CheckpointsPath()).Error("Failed to open dir")
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			log.WithFields(log.Fields{
				"name": file.Name(),
			}).Debug("Unexpected entry in checkpoint directory")
			continue
		}
		log.WithField("name", file.Name()).Trace("Found ckpt")

		c.checkpoints = append(c.checkpoints, &checkpoint{
			id:        file.Name(),
			container: c,
			last:      c.latestCheckpoint(),
		})
	}

	return nil
}

func (c *checkpoint) Rank() Rank {
	return c.container.Rank()
}

func (c *checkpoint) ID() string {
	return c.id
}

func (c *checkpoint) ContainerID() string {
	return c.container.ID()
}

func (c *checkpoint) Args() []string {
	return c.container.Args()
}

func (c *checkpoint) StatePath() string {
	return c.container.StatePath()
}

func (c *checkpoint) Path() string {
	return path.Join(c.container.CheckpointsPath(), c.ID())
}

func (c *checkpoint) PathAbs() string {
	return c.container.PathAbs(c.Path())
}

func (c *checkpoint) Dump(preDump bool) error {
	criuOpts := &libcontainer.CriuOpts{
		ImagesDirectory:   c.PathAbs(),
		LeaveRunning:      false,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_SOFT,
	}

	if preDump {
		criuOpts.LeaveRunning = true
		criuOpts.PreDump = true
	}

	err := c.container.Checkpoint(criuOpts)
	if err != nil {
		log.WithError(err).Error("Failed to checkpoint")
		return err
	}

	return nil
}

func (c *checkpoint) Restore(process *libcontainer.Process) error {
	criuOpts := &libcontainer.CriuOpts{
		ImagesDirectory:         c.PathAbs(),
		ParentImage:             c.last.PathAbs(),
		LeaveRunning:            true,
		TcpEstablished:          true,
		ShellJob:                true,
		FileLocks:               true,
		External:                c.container.external,
		ExternalUnixConnections: true,
		ManageCgroupsMode:       libcontainer.CRIU_CG_MODE_SOFT,
	}

	log.WithFields(log.Fields{
		"image":  criuOpts.ImagesDirectory,
		"parent": criuOpts.ParentImage,
	}).Debug("Restoring checkpoint")

	err := c.container.Restore(process, criuOpts)

	return err
}
