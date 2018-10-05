package scheduler

import (
	"fmt"
	"net/rpc"

	"github.com/spf13/viper"

	"github.com/planetA/konk/pkg/util"
	"github.com/planetA/konk/pkg/daemon"
)

func Run() error {
	listener, err := util.CreateListener(viper.GetInt("scheduler.port"))
	if err != nil {
		return err
	}
	defer listener.Close()

	sched := NewSchedulerServer()
	rpc.Register(sched)

	if err := util.ServerLoop(listener); err != nil {
		return err
	}

	return nil
}

// The scheduler coordinates the migration process between container process and a node-daemon.
// First, the scheduler connects to a node-daemon and receives a port number. Then, the scheduler
// connects to a container-process, tells it the destination (hostname and port number), and asks
// to do the migration. Port of the source should correspond to the container-process of
// the container we want to migrate.
func Migrate(destHost, srcHost string, srcPort int) error {
	destClient, err := daemon.NewDaemonClient(destHost, viper.GetInt("daemon.port"))
	if err != nil {
		return fmt.Errorf("Failed to reach node-daemon: %v", err)
	}
	defer destClient.Close()

	destPort, err := destClient.Receive()
	if err != nil {
		return fmt.Errorf("Node-daemon is not receiving: %v", err)
	}

	// Once we have the port number, we can tell the container-process to migrate the container
	srcClient, err := daemon.NewDaemonClient(srcHost, srcPort)
	if err != nil {
		return fmt.Errorf("Failed to reach container-process: %v", err)
	}
	defer srcClient.Close()

	err = srcClient.Migrate(destHost, destPort)
	if err != nil {
		return fmt.Errorf("Container-process did not migrate: %v", err)
	}

	return nil
}
