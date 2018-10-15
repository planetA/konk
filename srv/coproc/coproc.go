// Container-process service
package coproc

import (
	"log"
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

	pid, err := createContainer(id);
	if  err != nil {
		return err
	}

	cont, err := container.ContainerAttachPid(pid)
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

	log.Println("Launched command. Now waiting")

	cmd.Wait()

	return nil
}

// Connect to the local nymph daemon and inform it about the new container
func createContainer(id container.Id) (int, error) {
	nymph, err := nymph.NewClient("localhost")
	if err != nil {
		return -1, fmt.Errorf("Failed to connect to nymph: %v", err)
	}
	defer nymph.Close()

	pid, err := nymph.CreateContainer(id)
	if err != nil {
		return -1, fmt.Errorf("Container creation failed: %v", err)
	}

	return pid, nil
}
