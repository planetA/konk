// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"fmt"
	"os"
	"path"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	log "github.com/sirupsen/logrus"
)

type Rank int

type StartType int

const (
	Start StartType = iota
	Restore
)

const (
	containersDir  = "containers"
	checkpointsDir = "checkpoints"
	factoryDir     = "factory"
	stateFilename  = "state.json"
)

type Container struct {
	libcontainer.Container
	rank      Rank
	nymphRoot string
	args      []string
	external  []string
	tty       *tty

	checkpoints      []Checkpoint
	nextCheckpointId int
}

func newContainer(libCont libcontainer.Container, rank Rank, args []string, nymphRoot string) (*Container, error) {
	return &Container{
		Container:        libCont,
		rank:             rank,
		nymphRoot:        nymphRoot,
		args:             args,
		checkpoints:      make([]Checkpoint, 0),
		nextCheckpointId: 0,
	}, nil
}

func (c *Container) Rank() Rank {
	return c.rank
}

func (c *Container) Args() []string {
	return c.args
}

func (c *Container) AddExternal(external []string) {
	c.external = append(c.external, external...)
}

func (c *Container) PathAbs(pathRel string) string {
	return path.Join(c.nymphRoot, pathRel)
}

func (c *Container) ContainerPath() string {
	return path.Join(containersDir, factoryDir, c.ID())
}

func (c *Container) StateFilename() string {
	return stateFilename
}

func (c *Container) StatePath() string {
	return path.Join(c.ContainerPath(), stateFilename)
}

func (c *Container) StatePathAbs() string {
	return c.PathAbs(c.StatePath())
}

func (c *Container) CheckpointsPath() string {
	return path.Join(checkpointsDir, c.ID())
}

func (c *Container) Base() string {
	return c.nymphRoot
}

func (c *Container) NewProcess(args []string) (*libcontainer.Process, error) {
	process := &libcontainer.Process{
		Args: args,
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		User: "root",
		Init: true,
	}

	return process, nil
}

// setupIO modifies the given process config according to the options.
func setupIO(process *libcontainer.Process, rootuid, rootgid int) (*tty, error) {
	process.Stdin = nil
	process.Stdout = nil
	process.Stderr = nil
	t := &tty{}
	parent, child, err := utils.NewSockPair("console")
	if err != nil {
		return nil, err
	}
	process.ConsoleSocket = child
	t.postStart = append(t.postStart, parent, child)
	t.consoleC = make(chan error, 1)
	go func() {
		if err := t.recvtty(process, parent); err != nil {
			t.consoleC <- err
		}
		t.consoleC <- nil
	}()
	return t, nil
}

func (c *Container) latestCheckpoint() (Checkpoint, error) {
	if len(c.checkpoints) < 1 {
		return nil, fmt.Errorf("No checkpoints")
	}

	last := len(c.checkpoints) - 1
	return c.checkpoints[last], nil
}

func (c *Container) Launch(startType StartType) error {
	process, err := c.NewProcess(c.Args())
	if err != nil {
		return fmt.Errorf("Failed to create new process", err)
	}

	rootuid, err := c.Config().HostRootUID()
	if err != nil {
		return err
	}
	rootgid, err := c.Config().HostRootGID()
	if err != nil {
		return err
	}

	c.tty, err = setupIO(process, rootuid, rootgid)
	if err != nil {
		return fmt.Errorf("Failed to setup IO", err)
	}

	log.WithFields(log.Fields{
		"start_type":    startType,
		"containerRank": c.Rank(),
		"args":          c.Args(),
	}).Info("Launching process inside a container")

	switch startType {
	case Start:
		if err := c.Run(process); err != nil {
			log.Info(err)
			return fmt.Errorf("Failed to launch container in a process", err)
		}
	case Restore:
		checkpoint, err := c.latestCheckpoint()
		if err != nil {
			return err
		}

		err = checkpoint.Restore(process)
		if err != nil {
			log.WithFields(log.Fields{
				"ckpt":  checkpoint.PathAbs(),
				"args":  checkpoint.Args(),
				"error": err,
			}).Error("Restore failed")
			return err
		}
	}

	go func() {
		ret, err := process.Wait()
		if err != nil {
			log.Error("Waiting for process failed", err)
		}

		log.WithField("return", ret).Trace("Finished process")
	}()

	return nil
}

func (c *Container) Destroy() (err error) {
	log.WithField("rank", c.Rank()).Debug("Destroying container")

	err = c.Signal(os.Kill, true)
	log.WithError(err).WithFields(log.Fields{
		"rank": c.Rank(),
		"id":   c.ID(),
	}).Debug("Destroying container")

	if c.tty != nil {
		err = c.tty.Close()
	}

	cerr := c.Container.Destroy()
	if err != nil {
		return err
	}

	return cerr
}
