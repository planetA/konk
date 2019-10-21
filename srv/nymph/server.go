package nymph

import (
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"bytes"
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/util"

	. "github.com/planetA/konk/pkg/nymph"
)

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	reaper            *Reaper
	containerIds      map[int]container.Id // Map of PIDs to container Ids
	coordinatorClient *coordinator.Client
	containerFactory  libcontainer.Factory

	containerMutex *sync.Mutex
	containers     map[container.Id]libcontainer.Container

	imagesMutex *sync.Mutex
	images      map[string]*container.Image

	tmpDir string
}

func NewNymph() (*Nymph, error) {
	tmpDir := config.GetString(config.NymphTmpDir)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"path": tmpDir,
		}).Info("Temp directory already exists. Purging.")
		os.RemoveAll(tmpDir)
	}
	log.WithFields(log.Fields{
		"path": tmpDir,
	}).Trace("Creating temporary directory")

	if err := os.MkdirAll(tmpDir, 0770); err != nil {
		return nil, fmt.Errorf("Failed to create temporary directory %v: %v", tmpDir, err)
	}

	reaper, err := NewReaper()
	if err != nil {
		return nil, fmt.Errorf("NewReper: %v", err)
	}

	coord, err := coordinator.NewClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the coordinator: %v", err)
	}

	containersPath := path.Join(tmpDir, "containers")
	factory, err := libcontainer.New(containersPath, libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		return nil, fmt.Errorf("Failed to create container factory: %v", err)
	}

	nymph := &Nymph{
		reaper:            reaper,
		containerIds:      make(map[int]container.Id),
		coordinatorClient: coord,
		containerFactory:  factory,
		containerMutex:    &sync.Mutex{},
		containers:        make(map[container.Id]libcontainer.Container),
		imagesMutex:       &sync.Mutex{},
		images:            make(map[string]*container.Image),
		tmpDir:            tmpDir,
	}

	go func() {
		for {
			pid, more := <-reaper.deadChildren
			if !more {
				log.Println("Reaper died")
				return
			}

			if id, ok := nymph.forgetContainerPid(pid); ok {
				log.Println("Unregistering the container", id)
				err := nymph.coordinatorClient.UnregisterContainer(id)
				if err != nil {
					log.Fatal("Failed to unregister the container: ", err)
				}
			}
		}
	}()

	return nymph, nil
}

func (n *Nymph) getContainer(id container.Id) (libcontainer.Container, bool) {
	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	cont, ok := n.containers[id]

	return cont, ok
}

func (n *Nymph) rememberContainer(id container.Id, cont libcontainer.Container) {
	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	n.containers[id] = cont
	log.Println(n.containers)
}

func (n *Nymph) forgetContainerId(id container.Id) (int, bool) {
	log.Println("forgetContainerId", id)

	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	// cont, ok := n.containers[id]
	// if !ok {
	// 	return -1, false
	// }

	panic("Unimplemented")

	// pid := cont.Init.Proc.Pid
	delete(n.containers, id)
	// delete(n.containerIds, pid)

	return 0, true
}

func (n *Nymph) forgetContainerPid(pid int) (container.Id, bool) {
	log.Println("forgetContainerPid", pid)

	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	id, ok := n.containerIds[pid]
	if ok {
		delete(n.containers, id)
		delete(n.containerIds, pid)
	}

	return id, ok
}

// Container receiving server has only one method
func (n *Nymph) PrepareReceive(args *ReceiveArgs, reply *int) error {
	log.Println("Received a request to receive a checkpoint")

	// Passing port zero will make the kernel arbitrary free port
	listener, err := util.CreateListener(0)
	if err != nil {
		*reply = -1
		return fmt.Errorf("Failed to create a listener")
	}
	*reply = listener.Addr().(*net.TCPAddr).Port

	go func() {
		defer listener.Close()

		log.Println("Receiver is preparing for the migration. Start listening.")
		cont, err := criu.ReceiveListener(listener)
		if err != nil {
			log.Panicf("Connection failed: %v", err)
		}

		panic("Unimplemented")
		// n.rememberContainer(cont)

		hostname, err := os.Hostname()
		if err != nil {
			log.Panicf("Failed to get hostname: %v", err)
		}

		err = n.coordinatorClient.RegisterContainer(cont.Id, hostname)
		if err != nil {
			log.Panicf("Registering at the coordinator failed: %v", err)
		}
	}()

	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	// address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	// container, _ := n.getContainer(args.ContainerId)
	panic("Unimplemented")
	// if err := criu.Migrate(container, address); err != nil {
	// 	*reply = false
	// 	return err
	// }

	n.forgetContainerId(args.ContainerId)
	*reply = true
	return nil
}

func (n *Nymph) getImage(imagePath string) (*container.Image, error) {
	name := container.ImageName(imagePath)

	n.imagesMutex.Lock()
	log.WithFields(log.Fields{
		"path": imagePath,
		"name": name,
	}).Trace("Enter getImage")

	image, ok := n.images[name]

	defer log.WithFields(log.Fields{
		"path": imagePath,
		"name": name,
		"ok":   ok,
		"map":  n.images,
	}).Trace("Leave getImage")
	defer n.imagesMutex.Unlock()

	if ok {
		return image, nil
	}

	image, err := container.NewImage(n.tmpDir, imagePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open a container image %v: %v", imagePath, err)
	}

	n.images[name] = image

	return image, nil
}

func (n *Nymph) createContainer(id container.Id, imagePath string) (libcontainer.Container, error) {
	image, err := n.getImage(imagePath)
	if err != nil {
		log.WithFields(log.Fields{
			"image_path": imagePath,
			"err": err,
		}).Panic("Didn't get the image")
		return nil, err
	}

	config, err := instantiateConfig(image)
	if err != nil {
		return nil, fmt.Errorf("Failed to create container config: %v", err)
	}

	cont, err := n.containerFactory.Create(image.Name, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a container: %v", err)
	}

	n.rememberContainer(id, cont)

	return cont, nil
}

// Nymph creates a container, starts an init process inside and reports about the new container
// to the coordinator. The function replies with a path to the init container derictory
// Other processes need to attach to the init container using the path.
func (n *Nymph) CreateContainer(args CreateContainerArgs, path *string) error {
	cont, err := n.createContainer(args.Id, args.Image)
	if err != nil {
		log.WithFields(log.Fields{
			"container": args.Id,
			"error":     err,
		}).Error("Container creation failed")
		return fmt.Errorf("Container creation failed: %v", err)
	}

	log.WithField("cont", cont).Debug("Created container")
	// cont.Network, err = container.NewNetwork(cont.Id, cont.Path)
	// if err != nil {
	// 	return fmt.Errorf("Configuring network failed: %v", err)
	// }

	// Remember the container object

	panic("Unimplemented")

	// Return the path to the container to the launcher
	// *path = cont.Path
	return nil
}

// The nymph is notified that the process has been launched in the container, so the init process
// can start waiting.
func (n *Nymph) NotifyProcess(args NotifyProcessArgs, reply *bool) error {
	// cont, ok := n.getContainer(args.Id)
	// if !ok {
	// 	return fmt.Errorf("Container %v is not known\n", args.Id)
	// }

	panic("Unimplemented")
	// err := cont.Notify()
	// if err != nil {
	// 	return fmt.Errorf("Notifying the init process failed: %v", err)
	// }

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostname: %v", err)
	}

	err = n.coordinatorClient.RegisterContainer(args.Id, hostname)
	if err != nil {
		return fmt.Errorf("Registering at the coordinator failed: %v", err)
	}

	return nil
}

func (n *Nymph) Signal(args SignalArgs, reply *bool) error {
	log.WithField("args", args).Debug("Received signal")

	// var err error

	// cont, ok := n.getContainer(args.Id)
	// if !ok {
	// 	return fmt.Errorf("Receiver %v is not known\n", args.Id)
	// }

	panic("Unimplemented")
	// err = cont.Signal(args.Signal)
	// if err != nil {
	// 	return fmt.Errorf("Notifying the init process %v failed: %v", args.Id, err)
	// }

	return nil
}

func (n *Nymph) Run(args RunArgs, reply *bool) error {
	cont, err := n.createContainer(args.Id, args.Image)
	if err != nil {
		log.WithFields(log.Fields{
			"container": args.Id,
			"error":     err,
		}).Error("Container creation failed")
		return fmt.Errorf("Container creation failed: %v", err)
	}

	log.WithFields(log.Fields{
		"containerId": args.Id,
		"image":       args.Image,
		"args":        args.Args,
		"container":   cont,
	}).Info("Failed to launch container")

	return nil
}

func (n *Nymph) registerNymphOnce() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostaname: %v", err)
	}

	if err := n.coordinatorClient.RegisterNymph(hostname); err != nil {
		return fmt.Errorf("Failed to register nymph: %v", err)
	}

	return nil
}

func (n *Nymph) registerNymph() error {
	for {
		if err := n.registerNymphOnce(); err != nil {
			log.Println("Registration has failed: %v", err)
		} else {
			return nil
		}

		time.Sleep(5 * time.Second)
		log.Println("Trying to register once again")
	}
}

func (n *Nymph) unregisterNymph() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to get hostaname: %v", err)
	}

	if err := n.coordinatorClient.UnregisterNymph(hostname); err != nil {
		log.Printf("Failed to unregister nymph: %v", err)
	}
}

func (n *Nymph) _Close() {
	n.containerMutex.Lock()
	for _, cont := range n.containers {
		cont.Destroy()
	}
	n.containerMutex.Unlock()

	n.imagesMutex.Lock()
	for _, image := range n.images {
		image.Close()
	}
	n.imagesMutex.Unlock()

	n.unregisterNymph()

	n.coordinatorClient.Close()
	n.reaper.Close()

	os.RemoveAll(n.tmpDir)
}
