package coordinator

import (
	"fmt"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

// The scheduler coordinates the migration process between container process and a node-daemon.
// First, the scheduler connects to a nymph daemon and receives a port number. Then, the scheduler
// connects to a container-process, tells it the destination (hostname and port number), and asks
// to do the migration. Port of the source should correspond to the container-process of
// the container we want to migrate.
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
