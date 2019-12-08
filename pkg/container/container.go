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

type ContainerRegister struct {
	Factory  libcontainer.Factory
	Mutex    *sync.Mutex
	NymphDir string // Root directory of container register
	reg      map[Rank]*Container
}

func NewContainerRegister(nymphDir string) *ContainerRegister {
	rootDir := path.Join(nymphDir, containersDir)
	c := &ContainerRegister{
		NymphDir: nymphDir,
		Mutex:    &sync.Mutex{},
		reg:      make(map[Rank]*Container),
	}

	var err error
	c.Factory, err = libcontainer.New(c.PathAbs(), libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		log.Panicf("Failed to create container factory: %v", err)
	}

	log.WithFields(log.Fields{
		"containers": rootDir,
		"nymph":      nymphDir,
	}).Trace("Created container factory")

	if err := os.MkdirAll(c.CheckpointsPathAbs(), os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"dir": c.CheckpointsPathAbs(),
		}).Panic("Failed to create directory")
		return nil
	}

	return c
}

// Returns path to container register relative from nymph root
func (c *ContainerRegister) Path() string {
	return containersDir
}

// Returns absolute path to container register directory
func (c *ContainerRegister) PathAbs() string {
	return path.Join(c.NymphDir, c.Path())
}

func (c *ContainerRegister) CheckpointsPath() string {
	return checkpointsDir
}

func (c *ContainerRegister) CheckpointsPathAbs() string {
	return path.Join(c.NymphDir, c.CheckpointsPath())
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
		nymphRoot:      c.NymphDir,
		containerPath:  path.Join(containersDir, libCont.ID()),
		checkpointPath: path.Join(checkpointsDir, libCont.ID()),
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
		if cont.Init != nil {
			err := cont.Init.Signal(os.Kill)
			log.WithError(err).WithFields(log.Fields{
				"rank": cont.Rank(),
				"id":   cont.ID(),
			}).Debug("Destroying container")
		}

		if err := cont.Destroy(); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"rank": cont.Rank(),
				"id":   cont.ID(),
			}).Error("Destroying failed")
		}
	}
}
