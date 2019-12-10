package container

import (
	"path"

	"github.com/opencontainers/runc/libcontainer"
)

type Checkpoint interface {
	// Id of a checkpoint
	ID() string

	// Id of a container owning a checkpoint
	ContainerID() string

	Rank() Rank

	Dump() error

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
}

func (c *Container) NewCheckpoint() (Checkpoint, error) {
	checkpoint := &checkpoint{
		id:        "",
		container: c,
	}
	c.checkpoints = append(c.checkpoints)

	return checkpoint, nil
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
	return path.Join(c.container.CheckpointPath(), c.ID())
}

func (c *checkpoint) PathAbs() string {
	return path.Join(c.container.CheckpointPathAbs(), c.ID())
}

func (c *checkpoint) Dump() error {
	err := c.container.Checkpoint(&libcontainer.CriuOpts{
		ImagesDirectory:   c.PathAbs(),
		LeaveRunning:      false,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_SOFT,
	})

	return err
}
