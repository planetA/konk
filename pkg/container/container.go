// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
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
	containerPath  string
	checkpointPath string
	args           []string
	Init           *libcontainer.Process
}

func (c *Container) Rank() Rank {
	return c.rank
}

func (c *Container) Args() []string {
	return c.args
}

func (c *Container) CheckpointPath() string {
	return c.checkpointPath
}

func (c *Container) CheckpointPathAbs() string {
	return path.Join(c.nymphRoot, c.CheckpointPath())
}

func (c *Container) StatePath() string {
	return path.Join(c.containerPath, stateFilename)
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
