// Container-process service
package coproc

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"runtime"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/scheduler"
	"github.com/planetA/konk/pkg/util"
	"github.com/planetA/konk/pkg/daemon"
)

func Run(id int, args []string) error {
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

	daemon := daemon.NewCoSender(cmd.Process.Pid)
	rpc.Register(daemon)

	go func() {
		if err := util.ServerLoop(listener); err != nil {
			log.Panicf("XXX: server failed: %v", err)
		}
	}()

	// The port the scheduler should connect to ask the container process to send a checkpoint
	port := listener.Addr().(*net.TCPAddr).Port
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostname: %v", err)
	}

	// Report the port the node-daemon listens on
	sched, err := scheduler.NewSchedulerClient()
	if err != nil {
		return fmt.Errorf("Failed to connect to the scheduler: %v", err)
	}

	err = sched.ContainerAnnounce(id, hostname, port)
	if err != nil {
		return fmt.Errorf("Container announcement failed: %v", err)
	}
	sched.Close()

	cmd.Wait()

	return nil
}
