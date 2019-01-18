// Struct representing init process
package container

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/initial"
)

// Type representing a container init process controlled by nymph
type InitProc struct {
	Proc   *os.Process
	Cmd    *exec.Cmd
	Socket *os.File
}

func NewInitProc(id Id) (*InitProc, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to create socket pair: %v", err)
	}

	outerSocket := os.NewFile(uintptr(fds[0]), "outer")
	innerSocket := os.NewFile(uintptr(fds[1]), "inner")
	defer innerSocket.Close()

	// In future, potentially, run will return the container path. For now we just construct it
	cmd, err := initial.Run(innerSocket)
	if err != nil {
		return nil, fmt.Errorf("Failed to start init process: %v", err)
	}

	return &InitProc{
		Cmd:    cmd,
		Socket: outerSocket,
	}, nil
}

func NewInitAttach(pid int) (*InitProc, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to find init by PID: %v", err)
	}

	return &InitProc{
		Proc: proc,
	}, nil
}

type InitArgs struct {
	Root   string
	Name   string
	Id     Id
	Mounts []MountPoint
}

func (i *InitProc) sendParameters(id Id) error {
	encoder := json.NewEncoder(i.Socket)
	return encoder.Encode(InitArgs{
		Root: config.GetString(config.ContainerRootDir),
		Name: config.GetString(config.ContainerBaseName),
		Id:   id,
		Mounts: []MountPoint{
			// {"proc", "/proc", "proc"},
			{"tmp", "/tmp", "tmpfs"},
		},
	})
}

// Wait until init process reports it is ready to adopt a child
func (i *InitProc) waitInit() error {
	log.Println("Waiting init")
	const buf_size int = 20
	buf := make([]byte, buf_size)

	n, err := i.Socket.Read(buf)
	if err != nil {
		return fmt.Errorf("Wait init (read %v): %v", n, err)
	}

	pid, err := strconv.Atoi(string(buf[:n]))
	if err != nil {
		return fmt.Errorf("Failed to receive init PID: %v", err)
	}

	i.Proc, err = os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("Failed to find init by PID: %v", err)
	}

	return nil
}

// Notify the init process, that the application has started
func (i *InitProc) notify() error {
	log.Println("Notify init")

	// Need to write at least one byte
	dummy := make([]byte, 1)
	if n, err := i.Socket.Write(dummy); (n != 1) || (err != nil) {
		return fmt.Errorf("Notify init: %v", err)
	}

	return nil
}

func (i *InitProc) signal(signal syscall.Signal) error {
	log.Printf("Signal %v received\n", signal)

	i.Proc.Signal(signal)

	if i.Cmd != nil {
		i.Cmd.Process.Signal(signal)
	}

	return nil
}

func (i *InitProc) Close() {
	// Close the socket
	i.Socket.Close()

	i.Proc.Signal(os.Interrupt)

	// Kill container init process
	if i.Cmd != nil {
		i.Cmd.Process.Kill()
	}
}
