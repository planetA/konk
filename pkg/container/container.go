// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"fmt"
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
	stateFilename  = "state.json"
)

type Container struct {
	libcontainer.Container
	rank      Rank
	nymphRoot string
	args      []string
	external  []string
	tty       *tty
	Init      *libcontainer.Process
}

func newContainer(libCont libcontainer.Container, rank Rank, args []string, nymphRoot string) (*Container, error) {
	return &Container{
		Container: libCont,
		rank:      rank,
		nymphRoot: nymphRoot,
		args:      args,
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

type ContainerRegistrator func(Rank) error

func (c *Container) Launch(startType StartType, cr ContainerRegistrator) error {
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
		err = c.Restore(process, &libcontainer.CriuOpts{
			ImagesDirectory:         c.CheckpointPathAbs(),
			LeaveRunning:            true,
			TcpEstablished:          true,
			ShellJob:                true,
			FileLocks:               true,
			External:                c.external,
			ExternalUnixConnections: true,
			ManageCgroupsMode:       libcontainer.CRIU_CG_MODE_SOFT,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"ckpt":  c.CheckpointPathAbs(),
				"args":  c.Args(),
				"error": err,
			}).Error("Restore failed")
			return err
		}
	}

	if err := cr(c.Rank()); err != nil {
		return fmt.Errorf("Registering at the coordinator failed: %v", err)
	}

	return nil
}

func (c *Container) Destroy() (err error) {
	if c.tty != nil {
		err = c.tty.Close()
	}

	cerr := c.Container.Destroy()
	if err != nil {
		return err
	}

	return cerr
}

