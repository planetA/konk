package container

import (
	"fmt"
	"syscall"
)

// Namespace handle
type Handle int

func openNamespacePath(nsPath string) (Handle, error) {
	fd, err := syscall.Open(nsPath, syscall.O_RDONLY, 0)
	if err != nil {
		return -1, err
	}

	handle := Handle(fd)
	handle.CloseOnExec()

	return handle, nil
}

// UniqueId returns a string which uniquely identifies the namespace
// associated with the network handle.
func (handle Handle) UniqueId() string {
	var s syscall.Stat_t
	if handle == -1 {
		return "NS(none)"
	}
	if err := syscall.Fstat(int(handle), &s); err != nil {
		return "NS(unknown)"
	}
	return fmt.Sprintf("NS(%d:%d)", s.Dev, s.Ino)
}

func (handle Handle) CloseOnExec() {
	syscall.CloseOnExec(int(handle))
}

// Close file descriptor of a network handle
func (handle Handle) Close() error {
	return syscall.Close(int(handle))
}
