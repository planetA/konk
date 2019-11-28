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

type Container interface {
	libcontainer.Container
	Rank() Rank
}

type konkContainer struct {
	libcontainer.Container
	rank Rank
}

func (c *konkContainer) Rank() Rank {
	return c.rank
}

type ContainerRegister struct {
	Factory libcontainer.Factory
	Mutex   *sync.Mutex
	reg     map[Rank]Container
}

func NewContainerRegister(tmpDir string) *ContainerRegister {
	containersPath := path.Join(tmpDir, "containers")
	factory, err := libcontainer.New(containersPath, libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		log.Panicf("Failed to create container factory: %v", err)
	}

	log.WithFields(log.Fields{
		"container_path": containersPath,
	}).Trace("Created container factory")

	return &ContainerRegister{
		Factory: factory,
		reg:     make(map[Rank]Container),
		Mutex:   &sync.Mutex{},
	}
}

func (c *ContainerRegister) GetUnlocked(rank Rank) (Container, error) {
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

func (c *ContainerRegister) GetOrCreate(rank Rank, name string, config *configs.Config) (Container, error) {
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
	cont = &konkContainer{
		libCont,
		rank,
	}

	// Remember container
	c.reg[rank] = cont

	log.WithFields(log.Fields{
		"cont": cont,
		"rank": rank,
	}).Debug("Create container")

	return cont, nil

}

func (c *ContainerRegister) Delete(rank Rank) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	cont, ok := c.reg[rank]
	if !ok {
		log.WithField("rank", rank).Panic("Container not found")
	}

	cont.Destroy()
	delete(c.reg, rank)
}

func (c *ContainerRegister) Destroy() {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	for _, cont := range c.reg {
		cont.Destroy()
	}
}
