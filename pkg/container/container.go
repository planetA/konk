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

type Container struct {
	Id   Id
	Path string
}

type ContainerRegister struct {
	Factory libcontainer.Factory
	Mutex   *sync.Mutex
	reg     map[Id]libcontainer.Container
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
		reg:     make(map[Id]libcontainer.Container),
		Mutex:   &sync.Mutex{},
	}
}

func (c *ContainerRegister) GetOrCreate(id Id, name string, config *configs.Config) (libcontainer.Container, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// Check if container exists already
	cont, ok := c.reg[id]
	if ok {
		return cont, nil
	}

	cont, err := c.Factory.Create(name, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a container: %v", err)
	}

	// Remember container
	c.reg[id] = cont

	return cont, nil

}

func (c *ContainerRegister) Delete(id Id) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	cont, ok := c.reg[id]
	if !ok {
		log.WithField("container-id", id).Panic("Container not found")
	}

	cont.Destroy()
	delete(c.reg, id)
}

func (c *ContainerRegister) Destroy() {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	for _, cont := range c.reg {
		cont.Destroy()
	}
}
