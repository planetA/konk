package coordinator

import (
	"fmt"
	log "github.com/sirupsen/logrus"

	. "github.com/planetA/konk/pkg/coordinator"
)

type Request struct {
	args interface{}
	err  chan error
}

type Control struct {
	locationDB *LocationDB
	nymphSet   *NymphSet
	requests   chan Request
}

func NewControl() *Control {
	return &Control{
		locationDB: NewLocationDB(),
		nymphSet:   NewNymphSet(),
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
		case *RegisterNymphArgs:
			err = c.registerNymphImpl(args)
		case *UnregisterNymphArgs:
			err = c.unregisterNymphImpl(args)
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
	c.locationDB.Set(args.Rank, Location{args.Hostname})
	log.Printf("Request to register: %v\n\t\t%v", args, c.locationDB.Dump().db)

	return nil
}

func (c *Control) unregisterImpl(args *UnregisterContainerArgs) error {
	curHost := Location{args.Hostname}
	if err := c.locationDB.Unset(args.Rank, curHost); err != nil {
		log.Println(err)
	}
	log.Printf("Request to unregister: %v -- %v\n\t\t%v", curHost, args, c.locationDB.Dump().db)

	return nil
}

func (c *Control) migrateImpl(args *MigrateArgs) error {
	log.WithFields(log.Fields{
		"rank":     args.Rank,
		"dest":     args.DestHost,
		"pre-dump": args.PreDump,
	}).Debug("Received a request to migrate")

	src, ok := c.locationDB.Get(args.Rank)
	if !ok {
		return fmt.Errorf("Container %v is not known", args.Rank)
	}

	if err := Migrate(args.Rank, src.Hostname, args.DestHost, args.PreDump); err != nil {
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	c.locationDB.Set(args.Rank, Location{args.DestHost})

	return nil
}

func (c *Control) signalImpl(args *SignalArgs) error {
	signal := args.Signal

	log.Printf("Received a signal notification: %v\n", signal)

	var anyErr error
	for rank, loc := range c.locationDB.Dump().db {
		log.Printf("Sending signal %v to %v\n", signal, rank)
		if err := Signal(rank, loc.Hostname, signal); err != nil {
			anyErr = err
		}
	}

	return anyErr
}

func (c *Control) registerNymphImpl(args *RegisterNymphArgs) error {
	c.nymphSet.Add(Location{args.Hostname})
	log.Printf("Registered a nymph: %v\n\t\t%v\n", args, c.nymphSet.GetNymphs())
	return nil
}

func (c *Control) unregisterNymphImpl(args *UnregisterNymphArgs) error {
	ok := c.nymphSet.Del(Location{args.Hostname})
	log.Printf("Unregistered a nymph: %v\n\t\t%v\n", args, c.nymphSet.GetNymphs())

	if !ok {
		return fmt.Errorf("Nymph was not registered")
	}

	return nil
}
