package nymph

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/util"

	. "github.com/planetA/konk/pkg/nymph"
)

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	locationDB map[container.Id]int
	containers map[container.Id]*container.Container
}

func NewNymph() *Nymph {
	return &Nymph{
		locationDB: make(map[container.Id]int),
		containers: make(map[container.Id]*container.Container),
	}
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
		err := criu.ReceiveListener(listener)
		if err != nil {
			log.Panicf("Connection failed: %v", err)
		}
	}()
	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	pid := n.locationDB[args.ContainerId]
	if err := criu.Migrate(pid, address); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

func (n *Nymph) Register(args RegisterArgs, reply *bool) error {

	containerId := args.Id
	containerPid := args.Pid

	n.locationDB[containerId] = containerPid
	if err := registerAtCoordinator(containerId); err != nil {
		return fmt.Errorf("Failed to register container at the coordinator: %v", err)
	}

	log.Println("XXX: Should attach to the container now")

	*reply = true
	return nil
}

// Forward the registration request from the container to the coordinator
// and tell it the location of the container
func registerAtCoordinator(id container.Id) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostname: %v", err)
	}

	coord, err := coordinator.NewClient()
	if err != nil {
		return fmt.Errorf("Failed to connect to the coordinator: %v", err)
	}
	defer coord.Close()

	err = coord.RegisterContainer(id, hostname)
	if err != nil {
		return fmt.Errorf("Container announcement failed: %v", err)
	}

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
	n.containers[args.Id] = cont

	if err := cont.ConfigureNetwork(); err != nil {
		return fmt.Errorf("Configuring network failed: %v", err)
	}

	// Return the path to the container to the launcher
	*path = cont.Path
	return nil
}

// The nymph is notified that the process has been launched in the container, so the init process
// can start waiting.
func (n *Nymph) NotifyProcess(args NotifyProcessArgs, reply *bool) error {
	return n.containers[args.Id].Notify()
}

func CloseNymph(n *Nymph) {
	for _, cont := range n.containers {
		cont.Close()
	}
}
