package nymph

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/specconv"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/network"

	. "github.com/planetA/konk/pkg/nymph"
)

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	coordinatorClient *coordinator.Client

	containers *container.ContainerRegister

	imagesMutex *sync.Mutex
	images      map[string]*container.Image

	network network.Network

	tmpDir   string
	hostname string
}

func (n *Nymph) imagesPath() string {
	return path.Join(n.tmpDir, "images")
}

func (n *Nymph) criuPath() string {
	return path.Join(n.tmpDir, "criu")
}

func (n *Nymph) createTmpDirs() error {
	if _, err := os.Stat(n.tmpDir); !os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"path": n.tmpDir,
		}).Info("Temp directory already exists. Purging.")
		os.RemoveAll(n.tmpDir)
	}
	log.WithFields(log.Fields{
		"path": n.tmpDir,
	}).Trace("Creating temporary directory")

	if err := os.MkdirAll(n.tmpDir, 0770); err != nil {
		return fmt.Errorf("Failed to create temporary directory %v: %v", n.tmpDir, err)
	}

	if err := os.MkdirAll(n.imagesPath(), 0770); err != nil {
		return fmt.Errorf("Failed to create temporary directory %v: %v", n.imagesPath(), err)
	}

	if err := os.MkdirAll(n.criuPath(), 0770); err != nil {
		return fmt.Errorf("Failed to create temporary directory %v: %v", n.criuPath(), err)
	}

	return nil
}

func NewNymph() (*Nymph, error) {
	nymph := &Nymph{
		imagesMutex:       &sync.Mutex{},
		images:            make(map[string]*container.Image),
	}

	var err error
	nymph.hostname, err = os.Hostname()
	if err != nil {
		nymph._Close()
		return nil, fmt.Errorf("Failed to get hostname: %v", err)
	}

	networkType := config.GetString(config.NymphNetwork)
	nymph.network, err = network.New(networkType)
	if err != nil {
		log.WithField("network", networkType).Error("Failed to create network")
		nymph._Close()
		return nil, err
	}

	nymph.tmpDir = config.GetString(config.NymphTmpDir)
	if err := nymph.createTmpDirs(); err != nil {
		nymph._Close()
		return nil, err
	}

	nymph.containers = container.NewContainerRegister(nymph.tmpDir)

	nymph.coordinatorClient, err = coordinator.NewClient()
	if err != nil {
		nymph._Close()
		return nil, fmt.Errorf("Failed to connect to the coordinator: %v", err)
	}


	return nymph, nil
}

func (n *Nymph) forgetContainerId(id container.Id) (int, bool) {
	log.Println("forgetContainerId", id)

	n.containers.Delete(id)

	panic("Unimplemented")

	return 0, true
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

// Generate an image name from the container path
func ContainerName(imageName string, id container.Id) string {
	s := sha512.New512_256()
	s.Write([]byte(imageName))
	s.Write([]byte(fmt.Sprintf("%v", id)))
	return hex.EncodeToString(s.Sum(nil))
}

func addLabelId(config *configs.Config, id container.Id) {
	config.Labels = append(config.Labels, fmt.Sprintf("konk-id=%v", id))
}

func addLabelIpAddr(config *configs.Config, id container.Id) {
	addr := container.CreateContainerAddr(id)
	config.Labels = append(config.Labels, fmt.Sprintf("konk-ip=%v", addr.String()))
}

func (n *Nymph) addDevices(contConfig *configs.Config) error {
	caps := []string{
		"CAP_NET_ADMIN",
		"CAP_SYS_ADMIN",
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FSETID",
		"CAP_FOWNER",
		"CAP_MKNOD",
		"CAP_NET_RAW",
		"CAP_SETGID",
		"CAP_SETUID",
		"CAP_SETFCAP",
		"CAP_SETPCAP",
		"CAP_NET_BIND_SERVICE",
		"CAP_SYS_CHROOT",
		"CAP_KILL",
		"CAP_AUDIT_WRITE",
	}
	contConfig.Capabilities.Bounding = append(contConfig.Capabilities.Bounding, caps...)
	contConfig.Capabilities.Effective = append(contConfig.Capabilities.Effective, caps...)
	contConfig.Capabilities.Ambient = append(contConfig.Capabilities.Ambient, caps...)
	contConfig.Capabilities.Inheritable = append(contConfig.Capabilities.Inheritable, caps...)
	contConfig.Capabilities.Permitted = append(contConfig.Capabilities.Permitted, caps...)
	contConfig.Seccomp = &configs.Seccomp{
		DefaultAction: configs.Allow,
	}

	devicePath := config.GetString(config.ContainerDevicePath)
	dev, err := devices.GetDevices(devicePath)
	if err != nil {
		return err
	}

	contConfig.Devices = append(contConfig.Devices, dev...)
	contConfig.Cgroups = &configs.Cgroup{
		Name:   "test-container",
		Parent: "system",
		Resources: &configs.Resources{
			MemorySwappiness: nil,
			AllowAllDevices:  nil,
			AllowedDevices:   append(configs.DefaultAllowedDevices, contConfig.Devices...),
		},
	}
	return nil
}

func (n *Nymph) Run(args RunArgs, reply *bool) error {
	imagePath := args.Image

	image, err := n.getImage(imagePath)
	if err != nil {
		log.WithFields(log.Fields{
			"image_path": imagePath,
			"err":        err,
		}).Panic("Didn't get the image")
		return err
	}

	// TODO: cd to the bundle directory, see spec_linux.go
	contName := ContainerName(imagePath, args.Id)
	contConfig, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       contName,
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             image.Spec,
		RootlessEUID:     false,
		RootlessCgroups:  false,
	})
	if err != nil {
		return fmt.Errorf("Failed converting spec to config", err)
	}

	addLabelId(contConfig, args.Id)
	addLabelIpAddr(contConfig, args.Id)

	if err := n.network.InstallHooks(contConfig); err != nil {
		log.Error("Network specification failed")
		return err
	}

	if err := n.addDevices(contConfig); err != nil {
		log.Error("Adding devices failed")
		return err
	}

	cont, err := n.containers.GetOrCreate(args.Id, contName, contConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"container": args.Id,
			"error":     err,
		}).Error("Container creation failed")
		return fmt.Errorf("Container creation failed: %v", err)
	}

	userName := config.GetString(config.ContainerUsername)
	log.WithField("user", userName).Debug("Staring process")
	process := &libcontainer.Process{
		Args:   args.Args,
		Env:    []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		User:   "root",
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Init:   true,
	}

	log.WithFields(log.Fields{
		"containerId": args.Id,
		"image":       args.Image,
		"args":        args.Args,
		"container":   cont,
	}).Info("Launching process inside a container")

	if err := cont.Run(process); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to launch container in a process", err)
	}

	err = n.coordinatorClient.RegisterContainer(args.Id, n.hostname)
	if err != nil {
		return fmt.Errorf("Registering at the coordinator failed: %v", err)
	}

	ret, err := process.Wait()
	if err != nil {
		log.Info(err)
		return fmt.Errorf("Waiting for process failed", err)
	}

	log.WithField("return", ret).Trace("Finished process")

	return nil
}

func (n *Nymph) registerNymphOnce() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostname: %v", err)
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
		log.Printf("Failed to get hostname: %v", err)
	}

	if err := n.coordinatorClient.UnregisterNymph(hostname); err != nil {
		log.Printf("Failed to unregister nymph: %v", err)
	}
}

func (n *Nymph) _Close() {
	n.containers.Destroy()

	n.imagesMutex.Lock()
	for _, image := range n.images {
		image.Close()
	}
	n.imagesMutex.Unlock()

	n.unregisterNymph()

	n.coordinatorClient.Close()

	os.RemoveAll(n.tmpDir)

	n.network.Destroy()
}
