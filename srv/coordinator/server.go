package coordinator

import (
	"fmt"
	"log"
	"sync"

	"github.com/planetA/konk/pkg/container"

	. "github.com/planetA/konk/pkg/coordinator"
)

type Location struct {
	Hostname string
}

type Coordinator struct {
	locationMutex *sync.Mutex
	locationDB    map[container.Id]Location
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		locationMutex: &sync.Mutex{},
		locationDB:    make(map[container.Id]Location),
	}
}

func (c *Coordinator) RegisterContainer(args *RegisterContainerArgs, reply *bool) error {
	c.locationMutex.Lock()
	c.locationDB[args.Id] = Location{args.Hostname}

	log.Println(c.locationDB)
	c.locationMutex.Unlock()

	*reply = true
	return nil
}

// Coordinator can receive a migration request from an external entity.
func (c *Coordinator) Migrate(args *MigrateArgs, reply *bool) error {
	log.Printf("Received a request to move rank %v to %v\n", args.Id, args.DestHost)

	c.locationMutex.Lock()
	src := c.locationDB[args.Id]
	c.locationMutex.Unlock()

	if err := Migrate(args.Id, src.Hostname, args.DestHost); err != nil {
		*reply = false
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	*reply = true
	return nil
}

func (c *Coordinator) Signal(args *SignalArgs, anyErr *error) error {
	signal := args.Signal

	log.Printf("Received a signal notification: %v\n", signal)

	c.locationMutex.Lock()
	for id, loc := range c.locationDB {
		log.Printf("Sending signal %v to %v\n", signal, id)
		if err := Signal(id, loc.Hostname, signal); err != nil {
			*anyErr = err
		}
	}
	c.locationMutex.Unlock()

	return *anyErr
}
