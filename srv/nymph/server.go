package nymph

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/util"

	. "github.com/planetA/konk/pkg/nymph"
)

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	reaper         *Reaper
	containerMutex *sync.Mutex
	containers     map[container.Id]*container.Container
}

func NewNymph() (*Nymph, error) {
	reaper, err := NewReaper()
	if err != nil {
		return nil, fmt.Errorf("NewReper: %v", err)
	}

	return &Nymph{
		reaper:         reaper,
		containerMutex: &sync.Mutex{},
		containers:     make(map[container.Id]*container.Container),
	}, nil
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

		n.containerMutex.Lock()
		n.containers[cont.Id] = cont
		n.containerMutex.Unlock()
	}()
	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	n.containerMutex.Lock()
	container := n.containers[args.ContainerId]
	n.containerMutex.Unlock()
	if err := criu.Migrate(container, address); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

// Nymph creates a container, starts an init process inside and reports about the new container
// to the coordinator. The function replies with a path to the init container derictory
// Other processes need to attach to the init container using the path.
func (n *Nymph) CreateContainer(args CreateContainerArgs, path *string) error {
	cont, err := container.NewContainerInit(args.Id)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}

	// Remember the container object
	n.containerMutex.Lock()
	n.containers[args.Id] = cont
	n.containerMutex.Unlock()

	cont.Network, err = container.NewNetwork(cont.Id, cont.Path)
	if err != nil {
		return fmt.Errorf("Configuring network failed: %v", err)
	}

	// Return the path to the container to the launcher
	*path = cont.Path
	return nil
}

// The nymph is notified that the process has been launched in the container, so the init process
// can start waiting.
func (n *Nymph) NotifyProcess(args NotifyProcessArgs, reply *bool) error {
	var err error

	n.containerMutex.Lock()
	err = n.containers[args.Id].Notify()
	n.containerMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Notifying the init process failed: %v", err)
	}

	err = coordinator.Register(args.Id)
	if err != nil {
		return fmt.Errorf("Registering at the coordinator failed: %v", err)
	}

	return nil
}

func (n *Nymph) Signal(args SignalArgs, reply *bool) error {
	var err error

	n.containerMutex.Lock()
	err = n.containers[args.Id].Signal(args.Signal)
	n.containerMutex.Unlock()
	if err != nil {
		return fmt.Errorf("Notifying the init process %v failed: %v", args.Id, err)
	}

	return nil
}

func (n *Nymph) _Close() {
	n.containerMutex.Lock()
	for _, cont := range n.containers {
		cont.Close()
	}
	n.containerMutex.Unlock()

	n.reaper.Close()
}
