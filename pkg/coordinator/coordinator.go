package coordinator

import (
	"fmt"
	"os"
	"syscall"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

// The scheduler coordinates the migration process between container process and a node-daemon.
// First, the scheduler connects to the destination nymph and asks to prepare for receiving the
// checkpoint. The nymph returns the port number that should be used specifically for transferring
// this particular checkpoint. Then, the coordinator contacts the source nymph, tells it the
// destination hostname and port number, and asks to send the checkpoint.
func Migrate(containerId container.Id, srcHost, destHost string) error {
	destClient, err := nymph.NewClient(destHost)
	if err != nil {
		return fmt.Errorf("Failed to reach nymph process: %v", err)
	}
	defer destClient.Close()

	destPort, err := destClient.PrepareReceive()
	if err != nil {
		return fmt.Errorf("Nymph is not receiving: %v", err)
	}

	// Once we have the port number, we can tell the container-process to migrate the container
	srcClient, err := nymph.NewClient(srcHost)
	if err != nil {
		return fmt.Errorf("Failed to reach container-process: %v", err)
	}
	defer srcClient.Close()

	err = srcClient.Send(containerId, destHost, destPort)
	if err != nil {
		return fmt.Errorf("Container-process did not migrate: %v", err)
	}

	return nil
}

// Forward the registration request from the container to the coordinator
// and tell it the location of the container
func Register(id container.Id) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get hostname: %v", err)
	}

	coord, err := NewClient()
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

func Signal(containerId container.Id, host string, signal syscall.Signal) error {
	client, err := nymph.NewClient(host)
	if err != nil {
		return fmt.Errorf("Failed to connect to the nymph %v: %v", host, err)
	}
	defer client.Close()

	err = client.Signal(containerId, signal)
	if err != nil {
		return fmt.Errorf("Sending signal to the coordinator failed: %v", err)
	}

	return nil
}
