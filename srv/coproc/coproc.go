// Container-process service
package coproc

import (
	"fmt"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
	"github.com/planetA/konk/pkg/util"
)

func Run(id container.Id, args []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ctx, cancel := util.NewContext()
	defer cancel()

	cont, err := container.NewContainer(id)
	if err != nil {
		return fmt.Errorf("Failed to create a container: %v", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			cont.Delete()
		}
	}()

	cmd, err := cont.LaunchCommand(args)
	if err != nil {
		return err
	}

	// Start node-daemon server inside a container process
	listener, err := util.CreateListener(0)
	if err != nil {
		return err
	}
	defer listener.Close()

	registerAtNymph(id, cmd.Process.Pid)

	cmd.Wait()

	return nil
}

// Connect to the local nymph daemon and inform it about the new container
func registerAtNymph(id container.Id, pid int) error {
	nymph, err := nymph.NewClient("localhost")
	if err != nil {
		return fmt.Errorf("Failed to connect to nymph: %v", err)
	}
	defer nymph.Close()

	err = nymph.Register(id, pid)

	if err != nil {
		return fmt.Errorf("Container registeration failed: %v", err)
	}

	return nil
}
