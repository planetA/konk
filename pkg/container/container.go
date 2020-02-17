// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"fmt"
	"net"
	"os"
	"path"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/config"
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

func (c *Container) NewProcess(args []string, init bool) (*libcontainer.Process, error) {
	user, err := config.GetStringErr(config.ContainerUsername)
	if err != nil {
		return nil, err
	}

	process := &libcontainer.Process{
		Args: args,
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		User: user,
		Init: init,
	}

	return process, nil
}

// setupIO modifies the given process config according to the options.
func setupIO(process *libcontainer.Process, rootuid, rootgid int, detach bool, sockpath string) (*tty, error) {
	process.Stdin = nil
	process.Stdout = nil
	process.Stderr = nil
	t := &tty{}
	if !detach {
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
	} else {
		// the caller of runc will handle receiving the console master
		conn, err := net.Dial("unix", sockpath)
		if err != nil {
			return nil, err
		}
		uc, ok := conn.(*net.UnixConn)
		if !ok {
			return nil, fmt.Errorf("casting to UnixConn failed")
		}
		t.postStart = append(t.postStart, uc)
		socket, err := uc.File()
		if err != nil {
			return nil, err
		}
		t.postStart = append(t.postStart, socket)
		process.ConsoleSocket = socket
	}
	return t, nil
}

func (c *Container) latestCheckpoint() Checkpoint {
	if len(c.checkpoints) < 1 {
		return nil
	}

	last := len(c.checkpoints) - 1
	return c.checkpoints[last]
}

func (c *Container) Launch(startType StartType, args []string, init bool) error {
	process, err := c.NewProcess(args, init)
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

	detach := false
	sockpath := ""
	c.tty, err = setupIO(process, rootuid, rootgid, detach, sockpath)
	if err != nil {
		return fmt.Errorf("Failed to setup IO", err)
	}

	log.WithFields(log.Fields{
		"start_type":    startType,
		"containerRank": c.Rank(),
		"process":       process,
	}).Info("Launching process inside a container")

	switch startType {
	case Start:
		if err := c.Run(process); err != nil {
			log.WithFields(log.Fields{
				"process": process,
			}).WithError(err).Error("Failed to launch container in a process")
			return fmt.Errorf("Failed to launch container in a process", err)
		}
	case Restore:
		checkpoint := c.latestCheckpoint()
		if checkpoint == nil {
			return fmt.Errorf("No checkpoint")
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
