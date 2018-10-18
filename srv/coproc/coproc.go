// Container-process service
package coproc

import (
	"log"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

func Run(id container.Id, args []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cont, err := nymph.CreateContainer(id);
	if  err != nil {
		return err
	}

	cmd, err := cont.LaunchCommand(args)
	if err != nil {
		return err
	}

	log.Println("Launched command. Now waiting")

	cmd.Wait()

	return nil
}

