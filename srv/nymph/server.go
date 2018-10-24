package nymph

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/initial"
	"github.com/planetA/konk/pkg/util"

	. "github.com/planetA/konk/pkg/nymph"
)

// Type representing a container init process controlled by nymph
type InitProc struct {
	containerPath string
	cmd           *exec.Cmd
	socket        *os.File
}

// Type for the server state of the connection to a nymph daemon
type Nymph struct {
	locationDB map[container.Id]int
	initProcs  []*InitProc
}

func NewNymph() *Nymph {
	return &Nymph{
		locationDB: make(map[container.Id]int),
		initProcs:  make([]*InitProc, 0),
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
	containerId := args.Id

	var err error
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("Failed to create socket pair: %v", err)
	}

	outerSocket := os.NewFile(uintptr(fds[0]), "outer")
	innerSocket := os.NewFile(uintptr(fds[1]), "inner")
	defer innerSocket.Close()

	// In future, potentially, run will return the container path. For now we just construct it
	cmd, err := initial.Run(innerSocket)
	if err != nil {
		return err
	}

	root := "/var/run/konk"
	containerName := "konk"
	containerPath := fmt.Sprintf("%v/%v%v", root, containerName, containerId)

	encoder := json.NewEncoder(outerSocket)
	encoder.Encode(InitArgs{
		Root: root,
		Name: containerName,
		Id:   containerId,
	})

	initProc := newInitProc(containerPath, cmd, outerSocket)

	// Remember the init process
	n.initProcs = append(n.initProcs, initProc)

	if err := initProc.waitInit(outerSocket); err != nil {
		return fmt.Errorf("Init process was not ready (read %v): %v", n, err)
	}


	// Return the path to the container to the launcher
	*path = containerPath
	return nil
}

func CloseNymph(n *Nymph) {
	for _, initProc := range n.initProcs {
		initProc.Close()
	}
}

func newInitProc(path string, cmd *exec.Cmd, socket *os.File) *InitProc {
	return &InitProc{
		containerPath: path,
		cmd:           cmd,
		socket:        socket,
	}
}

// Wait until init process reports it is ready to adopt a child
func (i *InitProc) waitInit(socket *os.File) error {
	log.Println("Waiting init")
	result := make([]byte, 1)
	if n, err := i.socket.Read(result); (n != 1) || (err != nil) {
		return fmt.Errorf("Wait init: %v", err)
	}

	return nil
}

func (i *InitProc) Close() {
	// Close the socket
	i.socket.Close()

	// Kill container init process
	i.cmd.Process.Kill()

	// Delete container directory
	os.RemoveAll(i.containerPath)
}
