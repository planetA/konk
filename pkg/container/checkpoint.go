package container

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/opencontainers/runc/libcontainer"
	log "github.com/sirupsen/logrus"
)

type MigrationType int

const (
	Migrate MigrationType = iota
	PreDump
	WithPreDump
)

func (m MigrationType) String() string {
	switch m {
	case Migrate:
		return "migrate"
	case PreDump:
		return "pre-dump"
	case WithPreDump:
		return "migrate-with-pre-dump"
	default:
		panic("Unreachable")
	}
}

type Checkpoint interface {
	// Id of a checkpoint
	Generation() int

	// Id of a container owning a checkpoint
	ContainerID() string

	Rank() Rank

	// Dump process into checkpoint
	Dump() error

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

	ImageInfo() *ImageInfoArgs

	Destroy()

	PageServer() bool
}

type CheckpointArgs struct {
	Parent     Checkpoint
	PreDump    bool
	PageServer string
}

type checkpoint struct {
	CheckpointArgs
	generation int
	container  *Container
}

func (c *Container) getCheckpoint(generation int) (Checkpoint, error) {
	for _, ckpt := range c.checkpoints {
		if ckpt.Generation() == generation {
			return ckpt, nil
		}
	}

	return nil, fmt.Errorf("Checkpoint %v not found", generation)
}

func (c *Container) NewCheckpoint(args *CheckpointArgs) (Checkpoint, error) {
	checkpoint := &checkpoint{
		*args,
		c.nextCheckpointId,
		c,
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

func (c *Container) LoadCheckpoint(target int) error {
	ckptPath := path.Join(c.CheckpointsPath(), strconv.Itoa(target))
	_, err := ioutil.ReadDir(ckptPath)
	if err != nil {
		log.WithError(err).WithField("dir", ckptPath).Error("Failed to open dir")
		return err
	}

	c.checkpoints = append(c.checkpoints, &checkpoint{
		generation: target,
		container:  c,
	})

	return nil
}

func (c *checkpoint) Rank() Rank {
	return c.container.Rank()
}

func (c *checkpoint) Generation() int {
	return c.generation
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
	return path.Join(c.container.CheckpointsPath(), strconv.Itoa(c.Generation()))
}

func (c *checkpoint) PathAbs() string {
	return c.container.PathAbs(c.Path())
}

func (c *checkpoint) ImageInfo() *ImageInfoArgs {
	return &ImageInfoArgs{
		Rank:       c.Rank(),
		ID:         c.ContainerID(),
		Args:       c.Args(),
		Generation: c.generation,
	}
}

func (c *checkpoint) Dump() error {
	var cpsInfo libcontainer.CriuPageServerInfo
	if c.PageServer != "" {
		cpsInfo = libcontainer.CriuPageServerInfo{
			Address: c.PageServer,
			Port:    7624,
		}
	}

	criuOpts := &libcontainer.CriuOpts{
		ImagesDirectory:   c.PathAbs(),
		LeaveRunning:      false,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		PageServer:        cpsInfo,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_SOFT,
		LogLevel:          libcontainer.CRIU_LOG_DEBUG,
	}

	if c.PreDump {
		criuOpts.LeaveRunning = true
		criuOpts.PreDump = true
	}

	if c.Parent != nil {
		criuOpts.ParentImage = c.Parent.PathAbs()
	}

	err := c.container.Checkpoint(criuOpts)
	if err != nil {
		log.WithError(err).Error("Failed to checkpoint")
		return err
	}

	return nil
}

func (c *checkpoint) Restore(process *libcontainer.Process) error {
	var parent string
	if c.Parent != nil {
		parent = c.Parent.PathAbs()
	} else {
		parent = ""
	}

	criuOpts := &libcontainer.CriuOpts{
		ImagesDirectory:         c.PathAbs(),
		ParentImage:             parent,
		LeaveRunning:            true,
		TcpEstablished:          true,
		ShellJob:                true,
		FileLocks:               true,
		External:                c.container.external,
		ExternalUnixConnections: true,
		ManageCgroupsMode:       libcontainer.CRIU_CG_MODE_SOFT,
		LogLevel:                libcontainer.CRIU_LOG_DEBUG,
	}

	log.WithFields(log.Fields{
		"image":  criuOpts.ImagesDirectory,
		"parent": criuOpts.ParentImage,
	}).Debug("Restoring checkpoint")

	err := c.container.Restore(process, criuOpts)

	return err
}

func (c *checkpoint) Destroy() {
	os.RemoveAll(c.PathAbs())
}

func (c *checkpoint) IsPageServer() bool {
	return c.PageServer != ""
}
