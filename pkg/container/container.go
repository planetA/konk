// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"os"
	"path"

	"github.com/opencontainers/runc/libcontainer"
)

type Rank int

const (
	containersDir  = "containers"
	checkpointsDir = "checkpoints"
	stateFilename  = "state.json"
)

type Container struct {
	libcontainer.Container
	rank           Rank
	nymphRoot      string
	args           []string
	Init           *libcontainer.Process
}

func newContainer(libCont libcontainer.Container, rank Rank, args []string, nymphRoot string) *Container {
	return &Container{
		Container:      libCont,
		rank:           rank,
		nymphRoot:      nymphRoot,
		args:           args,
	}
}

func (c *Container) Rank() Rank {
	return c.rank
}

func (c *Container) Args() []string {
	return c.args
}

func (c *Container) ContainerPath() string {
	return path.Join(containersDir, c.ID())
}

func (c *Container) CheckpointPath() string {
	return path.Join(checkpointsDir, c.ID())
}

func (c *Container) CheckpointPathAbs() string {
	return path.Join(c.nymphRoot, c.CheckpointPath())
}

func (c *Container) StatePath() string {
	return path.Join(c.ContainerPath(), stateFilename)
}

func (c *Container) StatePathAbs() string {
	return path.Join(c.nymphRoot, c.StatePath())
}

func (c *Container) StateFilename() string {
	return stateFilename
}

func (c *Container) Base() string {
	return c.nymphRoot
}

func (c *Container) NewProcess(args []string) (*libcontainer.Process, error) {
	return &libcontainer.Process{
		Args:   args,
		Env:    []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		User:   "root",
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Init:   true,
	}, nil
}
