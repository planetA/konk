// Struct representing init process
package nymph

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// Type representing a container init process controlled by nymph
type InitProc struct {
	containerPath string
	cmd           *exec.Cmd
	socket        *os.File
}

func newInitProc(path string, cmd *exec.Cmd, socket *os.File) *InitProc {
	return &InitProc{
		containerPath: path,
		cmd:           cmd,
		socket:        socket,
	}
}

// Wait until init process reports it is ready to adopt a child
func (i *InitProc) waitInit() error {
	log.Println("Waiting init")
	result := make([]byte, 1)
	if n, err := i.socket.Read(result); (n != 1) || (err != nil) {
		return fmt.Errorf("Wait init: %v", err)
	}

	return nil
}

// Notify the init process, that the application has started
func (i *InitProc) notify() error {
	log.Println("Notify init")

	// Need to write at least one byte
	dummy := make([]byte, 1)
	if n, err := i.socket.Write(dummy); (n != 1) || (err != nil) {
		return fmt.Errorf("Notify init: %v", err)
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
