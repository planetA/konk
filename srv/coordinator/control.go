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

type Request struct {
	args interface{}
	err  chan error
}

type Control struct {
	locationDB map[container.Id]Location
	requests   chan Request
}

func NewControl() *Control {
	return &Control{
		locationDB: make(map[container.Id]Location),
		requests:   make(chan Request),
	}
}

func (c *Control) Start() {
	for req := range c.requests {
		var err error
		switch args := req.args.(type) {
		case *RegisterContainerArgs:
			err = c.registerImpl(args)
		case *UnregisterContainerArgs:
			err = c.unregisterImpl(args)
		case *MigrateArgs:
			err = c.migrateImpl(args)
		case *SignalArgs:
			err = c.signalImpl(args)
		default:
			log.Printf("Arg: %v %T\n", args, args)
			panic("Unknown argument")
		}

		req.err <- err
	}

	panic("Never reached")
}

func (c *Control) Request(args interface{}) error {
	errChan := make(chan error)
	c.requests <- Request{args, errChan}

	err := <-errChan
	if err != nil {
		return fmt.Errorf("Request to coordinator control failed: %v", err)
	}

	return nil
}

func (c *Control) registerImpl(args *RegisterContainerArgs) error {
	c.locationDB[args.Id] = Location{args.Hostname}
	log.Printf("Request to register: %v\n\t\t%v", args, c.locationDB)

	return nil
}

func (c *Control) unregisterImpl(args *UnregisterContainerArgs) error {
	curHost, ok := c.locationDB[args.Id]
	fromHost := Location{args.Hostname}
	if ok && curHost == fromHost {
		delete(c.locationDB, args.Id)
	}
	log.Printf("Request to unregister: %v (%v) -- %v\n\t\t%v", curHost, ok, args, c.locationDB)

	if !ok {
		return fmt.Errorf("Container %v was not registered", args.Id)
	}

	return nil
}

func (c *Control) migrateImpl(args *MigrateArgs) error {
	log.Printf("Received a request to move rank %v to %v\n", args.Id, args.DestHost)

	src := c.locationDB[args.Id]

	if err := Migrate(args.Id, src.Hostname, args.DestHost); err != nil {
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	return nil
}

func (c *Control) signalImpl(args *SignalArgs) error {
	signal := args.Signal

	log.Printf("Received a signal notification: %v\n", signal)

	var anyErr error
	for id, loc := range c.locationDB {
		log.Printf("Sending signal %v to %v\n", signal, id)
		if err := Signal(id, loc.Hostname, signal); err != nil {
			anyErr = err
		}
	}

	return anyErr
}
