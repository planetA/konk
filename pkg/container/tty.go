// +build linux

// File taken from github.com/opencontainers/runc/tty.go

package container

import (
	"io"
	"os"
	"sync"

	"github.com/containerd/console"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
)

type tty struct {
	epoller   *console.Epoller
	console   *console.EpollConsole
	stdin     console.Console
	closers   []io.Closer
	postStart []io.Closer
	wg        sync.WaitGroup
	consoleC  chan error
}

func (t *tty) copyIO(w io.Writer, r io.ReadCloser) {
	defer t.wg.Done()
	io.Copy(w, r)
	r.Close()
}

func inheritStdio(process *libcontainer.Process) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return nil
}

func (t *tty) recvtty(process *libcontainer.Process, socket *os.File) (Err error) {
	f, err := utils.RecvFd(socket)
	if err != nil {
		return err
	}
	cons, err := console.ConsoleFromFile(f)
	if err != nil {
		return err
	}
	console.ClearONLCR(cons.Fd())
	epoller, err := console.NewEpoller()
	if err != nil {
		return err
	}
	epollConsole, err := epoller.Add(cons)
	if err != nil {
		return err
	}
	defer func() {
		if Err != nil {
			epollConsole.Close()
		}
	}()
	go epoller.Wait()
	go io.Copy(epollConsole, os.Stdin)
	t.wg.Add(1)
	go t.copyIO(os.Stdout, epollConsole)

	// set raw mode to stdin and also handle interrupt
	// stdin, err := console.ConsoleFromFile(os.Stdin)
	// if err != nil {
	// 	return err
	// }
	// if err := stdin.SetRaw(); err != nil {
	// 	return fmt.Errorf("failed to set the terminal from the stdin: %v", err)
	// }

	t.epoller = epoller
	// t.stdin = stdin
	t.stdin = nil
	t.console = epollConsole
	t.closers = []io.Closer{epollConsole}
	return nil
}

func (t *tty) waitConsole() error {
	if t.consoleC != nil {
		return <-t.consoleC
	}
	return nil
}

// ClosePostStart closes any fds that are provided to the container and dup2'd
// so that we no longer have copy in our process.
func (t *tty) ClosePostStart() error {
	for _, c := range t.postStart {
		c.Close()
	}
	return nil
}

// Close closes all open fds for the tty and/or restores the original
// stdin state to what it was prior to the container execution
func (t *tty) Close() error {
	// ensure that our side of the fds are always closed
	for _, c := range t.postStart {
		c.Close()
	}
	// the process is gone at this point, shutting down the console if we have
	// one and wait for all IO to be finished
	if t.console != nil && t.epoller != nil {
		t.console.Shutdown(t.epoller.CloseConsole)
	}
	t.wg.Wait()
	for _, c := range t.closers {
		c.Close()
	}
	if t.stdin != nil {
		t.stdin.Reset()
	}
	return nil
}

func (t *tty) resize() error {
	if t.console == nil {
		return nil
	}
	return t.console.ResizeFrom(console.Current())
}
