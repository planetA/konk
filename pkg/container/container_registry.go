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

type ContainerRegister struct {
	Factory  libcontainer.Factory
	Mutex    *sync.Mutex
	NymphDir string // Root directory of container register
	reg      map[Rank]*Container
}

func NewContainerRegister(nymphDir string) *ContainerRegister {
	c := &ContainerRegister{
		NymphDir: nymphDir,
		Mutex:    &sync.Mutex{},
		reg:      make(map[Rank]*Container),
	}

	if err := os.MkdirAll(c.FactoryPathAbs(), os.ModeDir|os.ModePerm); err != nil {
		log.WithFields(log.Fields{
			"dir": c.FactoryPathAbs(),
		}).Panic("Failed to create directory")
		return nil
	}

	var err error
	c.Factory, err = libcontainer.New(c.FactoryPathAbs(), libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		log.Panicf("Failed to create container factory: %v", err)
	}

	log.WithFields(log.Fields{
		"factory": c.FactoryPath(),
		"nymph":   nymphDir,
	}).Trace("Created container factory")

	return c
}

func (c *ContainerRegister) FactoryPath() string {
	return path.Join(containersDir, factoryDir)
}

func (c *ContainerRegister) FactoryPathAbs() string {
	return path.Join(c.NymphDir, c.FactoryPath())
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

	cont, err = newContainer(libCont, rank, args, c.NymphDir)
	if err != nil {
		return nil, err
	}

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

	cont, err = newContainer(libCont, rank, args, c.NymphDir)
	if err != nil {
		return nil, err
	}

	if err := cont.LoadCheckpoints(); err != nil {
		return nil, err
	}

	// Remember container
	c.reg[rank] = cont

	log.WithFields(log.Fields{
		"cont": cont,
		"rank": rank,
	}).Debug("Load container")

	return cont, nil
}

func (c *ContainerRegister) DeleteUnlocked(rank Rank) {
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
		if err := cont.Destroy(); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"rank": cont.Rank(),
				"id":   cont.ID(),
			}).Error("Destroying failed")
		}
	}
}
