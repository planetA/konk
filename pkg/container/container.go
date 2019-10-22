// +build go1.10

// We depend on go1.10, because of the behaviour of LockOSThread behaviour.
// See: https://github.com/vishvananda/netns/issues/17

package container

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

type Container struct {
	Id      Id
	Path    string
	Network *Network
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

func (c *ContainerRegister) Create(id Id, imageName string, spec *specs.Spec) (libcontainer.Container, error) {
	name := ContainerName(imageName, id)

	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       name,
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             spec,
		RootlessEUID:     false,
		RootlessCgroups:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed converting spec to config", err)
	}

	log.WithFields(log.Fields{
		"image":     imageName,
		"container": name,
		"rootfs":    config.Rootfs,
	}).Debug("Creating a container from factory")

	cont, err := c.Factory.Create(name, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a container: %v", err)
	}

	return cont, nil
}

// Not protected by a lock
func (c *ContainerRegister) Get(id Id) (libcontainer.Container, bool) {
	cont, ok := c.reg[id]
	return cont, ok
}

// Not protected by a lock
func (c *ContainerRegister) Set(id Id, cont libcontainer.Container) {
	c.reg[id] = cont
	log.Println(c.reg)
}

func (c *ContainerRegister) GetOrCreate(id Id, imageName string, spec *specs.Spec) (libcontainer.Container, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	cont, ok := c.Get(id)
	if ok {
		return cont, nil
	}

	cont, err := c.Create(id, imageName, spec)
	if err != nil {
		return nil, err
	}

	c.Set(id, cont)

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

// Generate an image name from the container path
func ContainerName(imageName string, id Id) string {
	s := sha512.New512_256()
	s.Write([]byte(imageName))
	s.Write([]byte(fmt.Sprintf("%v", id)))
	return hex.EncodeToString(s.Sum(nil))
}
