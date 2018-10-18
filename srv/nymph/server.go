package nymph

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"golang.org/x/sys/unix"

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
// to the coordinator. The function replies with a path to the init container derictory
// Other processes need to attach to the init container using the path.
func (n *Nymph) CreateContainer(args CreateContainerArgs, path *string) error {
	containerId := args.Id

	var err error
	*path, err = createContainer(containerId)
	if err != nil {
		return fmt.Errorf("Failed to create container %v: %v", containerId, err)
	}

	return nil
}

// Create container and return the path to the container control directory
func createContainer(id container.Id) (string, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return "", fmt.Errorf("Failed to create socket pair: %v", err)
	}

	// In future, potentially, run will return the container path. For now we just construct it
	_, err = initial.Run(fds[1])
	if err != nil {
		return "", err
	}

	outerSocket := os.NewFile(uintptr(fds[0]), "outer")
	defer outerSocket.Close()
	innerSocket := os.NewFile(uintptr(fds[0]), "inner")
	defer innerSocket.Close()

	root := "/var/run/konk"
	containerName := fmt.Sprintf("konk%v", id)
	containerPath := fmt.Sprintf("%v/%v", root, containerName)

	encoder := json.NewEncoder(outerSocket)
	encoder.Encode(InitArgs{
		Root: root,
		Name: containerName,
		Id:   id,
	})

	log.Println("Waiting init")
	result := make([]byte, 1)
	if n, err := outerSocket.Read(result); (n != 1) || (err != nil) {
		return "", fmt.Errorf("Init process was not ready (read %v): %v", n, err)
	}

	return containerPath, nil
}
