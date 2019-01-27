package nymph

import (
	"fmt"
	"log"
	"net"
	"os"
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
	reaper            *Reaper
	containerMutex    *sync.Mutex
	containers        map[container.Id]*container.Container
	containerIds      map[int]container.Id // Map of PIDs to container Ids
	coordinatorClient *coordinator.Client
}

func NewNymph() (*Nymph, error) {
	reaper, err := NewReaper()
	if err != nil {
		return nil, fmt.Errorf("NewReper: %v", err)
	}

	coord, err := coordinator.NewClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the coordinator: %v", err)
	}

	nymph := &Nymph{
		reaper:            reaper,
		containerMutex:    &sync.Mutex{},
		containers:        make(map[container.Id]*container.Container),
		containerIds:      make(map[int]container.Id),
		coordinatorClient: coord,
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

func (n *Nymph) getContainer(id container.Id) (*container.Container, bool) {
	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	cont, ok := n.containers[id]

	return cont, ok
}

func (n *Nymph) rememberContainer(cont *container.Container) {
	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	n.containers[cont.Id] = cont
	n.containerIds[cont.Init.Proc.Pid] = cont.Id
	log.Println(n.containers)
}

func (n *Nymph) forgetContainerId(id container.Id) (int, bool) {
	log.Println("forgetContainerId", id)

	n.containerMutex.Lock()
	defer n.containerMutex.Unlock()

	cont, ok := n.containers[id]
	if !ok {
		return -1, false
	}

	pid := cont.Init.Proc.Pid
	delete(n.containers, id)
	delete(n.containerIds, pid)

	return pid, true
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

		n.rememberContainer(cont)

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

	address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	container, _ := n.getContainer(args.ContainerId)
	if err := criu.Migrate(container, address); err != nil {
		*reply = false
		return err
	}

	n.forgetContainerId(args.ContainerId)
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
	n.rememberContainer(cont)

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
	cont, ok := n.getContainer(args.Id)
	if !ok {
		return fmt.Errorf("Container %v is not known\n", args.Id)
	}

	err := cont.Notify()
	if err != nil {
		return fmt.Errorf("Notifying the init process failed: %v", err)
	}

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
	var err error

	cont, ok := n.getContainer(args.Id)
	if !ok {
		return fmt.Errorf("Receiver %v is not known\n", args.Id)
	}

	err = cont.Signal(args.Signal)
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

	n.coordinatorClient.Close()
	n.reaper.Close()
}
