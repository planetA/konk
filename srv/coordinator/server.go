package coordinator

import (
	"fmt"
	"log"

	"github.com/planetA/konk/pkg/container"

	. "github.com/planetA/konk/pkg/coordinator"
)

type Location struct {
	Hostname string
}

type Coordinator struct {
	locationDB map[container.Id]Location
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		locationDB: make(map[container.Id]Location),
	}
}

func (c *Coordinator) RegisterContainer(args *RegisterContainerArgs, reply *bool) error {
	c.locationDB[args.Id] = Location{args.Hostname}

	log.Println(c.locationDB)

	*reply = true
	return nil
}

// Coordinator can receive a migration request from an external entity.
func (c *Coordinator) Migrate(args *MigrateArgs, reply *bool) error {
	log.Printf("Received a request to move rank %v to %v\n", args.Id, args.DestHost)

	src := c.locationDB[args.Id]

	if err := Migrate(args.Id, src.Hostname, args.DestHost); err != nil {
		*reply = false
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	*reply = true
	return nil
}
