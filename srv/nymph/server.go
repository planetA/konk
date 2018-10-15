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
	"github.com/planetA/konk/pkg/initial"
	"github.com/planetA/konk/pkg/util"

	. "github.com/planetA/konk/pkg/nymph"
)

func NewNymph() *Nymph {
	return &Nymph{
		locationDB: make(map[container.Id]int),
	}
}

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	locationDB map[container.Id]int
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
// to the coordinator. The function replies with a pid of the init process, other processes need to attac to the init process.
func (n *Nymph) CreateContainer(args CreateContainerArgs, pid *int) error {
	containerId := args.Id

	var err error
	*pid, err = createContainer(containerId)
	if err != nil {
		return fmt.Errorf("Failed to create container %v: %v", containerId, err)
	}

	return nil
}

func createContainer(id container.Id) (int, error) {
	log.Println("XXX: Should create container now")

	pid, err := initial.Run(int(id))
	if err != nil {
		return -1, err
	}

	return pid, nil
}
