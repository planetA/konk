// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	log "github.com/sirupsen/logrus"
)

const (
	containersDir = "./containers"
	stateFilename = "state.json"
)

type Container struct {
	libcontainer.Container
	rank           Rank
	containerPath  string
	checkpointPath string
	args           []string
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

func (c *Container) StatePath() string {
	return path.Join(c.containerPath, stateFilename)
}

func (c *Container) StateFilename() string {
	return stateFilename
}

type ContainerRegister struct {
	Factory         libcontainer.Factory
	Mutex           *sync.Mutex
	ContainersPath  string
	CheckpointsPath string
	reg             map[Rank]*Container
}

func NewContainerRegister() *ContainerRegister {
	containersPath := path.Join("/tmp/konk/nymph", containersDir)
	factory, err := libcontainer.New(containersPath, libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		log.Panicf("Failed to create container factory: %v", err)
	}

	log.WithFields(log.Fields{
		"container_path": containersDir,
	}).Trace("Created container factory")

	checkpointsPath := path.Join(containersDir, "checkpoints")
	if err := os.MkdirAll(checkpointsPath, os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"dir": checkpointsPath,
		}).Panic("Failed to create directory")
		return nil
	}

	return &ContainerRegister{
		Factory:         factory,
		Mutex:           &sync.Mutex{},
		ContainersPath:  containersDir,
		CheckpointsPath: checkpointsPath,
		reg:             make(map[Rank]*Container),
	}
}

func (c *ContainerRegister) GetUnlocked(rank Rank) (*Container, error) {
	cont, ok := c.reg[rank]
	if ok {
		log.WithFields(log.Fields{
			"cont": cont,
			"rank": rank,
		}).Debug("Get container")
		return cont, nil
	}

	return nil, fmt.Errorf("Container %v not found", rank)
}

func (c *ContainerRegister) initContainer(libCont libcontainer.Container, rank Rank, args []string) *Container {
	return &Container{
		Container:      libCont,
		rank:           rank,
		containerPath:  path.Join(c.ContainersPath, libCont.ID()),
		checkpointPath: path.Join(c.CheckpointsPath, libCont.ID()),
		args:           args,
	}
}

func (c *ContainerRegister) GetOrCreate(rank Rank, name string, args []string, config *configs.Config) (*Container, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// Check if container exists already
	cont, ok := c.reg[rank]
	if ok {
		return cont, nil
	}

	libCont, err := c.Factory.Create(name, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a libcontainer container: %v", err)
	}
	cont = c.initContainer(libCont, rank, args)

	// Remember container
	c.reg[rank] = cont

	log.WithFields(log.Fields{
		"cont": cont,
		"rank": rank,
	}).Debug("Create container")

	return cont, nil
}

func (c *ContainerRegister) Load(rank Rank, name string, args []string) (*Container, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	log.WithFields(log.Fields{
		"rank": rank,
		"name": name,
	}).Trace("Loading container from checkpoint")

	// Check if container exists already
	cont, ok := c.reg[rank]
	if ok {
		return nil, fmt.Errorf("Container already loaded")
	}

	libCont, err := c.Factory.Load(name)
	if err != nil {
		return nil, fmt.Errorf("Failed to lead a libcontainer", err)
	}

	cont = c.initContainer(libCont, rank, args)

	// Remember container
	c.reg[rank] = cont

	log.WithFields(log.Fields{
		"cont": cont,
		"rank": rank,
	}).Debug("Load container")

	return cont, nil
}

func (c *ContainerRegister) Delete(rank Rank) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	cont, ok := c.reg[rank]
	if !ok {
		log.WithField("rank", rank).Panic("Container not found")
	}

	err := cont.Destroy()
	if err != nil {
		log.WithError(err).Error("Deleting container failed")
	}

	delete(c.reg, rank)
}

func (c *ContainerRegister) Destroy() {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	for _, cont := range c.reg {
		cont.Destroy()
	}
}
