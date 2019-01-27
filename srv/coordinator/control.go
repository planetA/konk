package coordinator

import (
	"fmt"
	"log"

	. "github.com/planetA/konk/pkg/coordinator"
)

type Request struct {
	args interface{}
	err  chan error
}

type Control struct {
	locationDB *LocationDB
	requests   chan Request
}

func NewControl() *Control {
	return &Control{
		locationDB: NewLocationDB(),
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
	c.locationDB.Set(args.Id, Location{args.Hostname})
	log.Printf("Request to register: %v\n\t\t%v", args, c.locationDB.Dump().db)

	return nil
}

func (c *Control) unregisterImpl(args *UnregisterContainerArgs) error {
	curHost := Location{args.Hostname}
	if err := c.locationDB.Unset(args.Id, curHost); err != nil {
		log.Println(err)
	}
	log.Printf("Request to unregister: %v -- %v\n\t\t%v", curHost, args, c.locationDB.Dump().db)

	return nil
}

func (c *Control) migrateImpl(args *MigrateArgs) error {
	log.Printf("Received a request to move rank %v to %v\n", args.Id, args.DestHost)

	src, ok := c.locationDB.Get(args.Id)
	if !ok {
		return fmt.Errorf("Container %v is not known", args.Id)
	}

	if err := Migrate(args.Id, src.Hostname, args.DestHost); err != nil {
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	return nil
}

func (c *Control) signalImpl(args *SignalArgs) error {
	signal := args.Signal

	log.Printf("Received a signal notification: %v\n", signal)

	var anyErr error
	for id, loc := range c.locationDB.Dump().db {
		log.Printf("Sending signal %v to %v\n", signal, id)
		if err := Signal(id, loc.Hostname, signal); err != nil {
			anyErr = err
		}
	}

	return anyErr
}
