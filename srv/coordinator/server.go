package coordinator

import (
	. "github.com/planetA/konk/pkg/coordinator"
)

type Coordinator struct {
	control *Control
}

func NewCoordinator(control *Control) *Coordinator {
	return &Coordinator{
		control: control,
	}
}

func (c *Coordinator) RegisterContainer(args *RegisterContainerArgs, reply *bool) error {
	if err := c.control.Request(args); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

// Delete the record about the container location in the database
func (c *Coordinator) UnregisterContainer(args *UnregisterContainerArgs, reply *bool) error {
	if err := c.control.Request(args); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

// Coordinator can receive a migration request from an external entity.
func (c *Coordinator) Migrate(args *MigrateArgs, reply *bool) error {
	if err := c.control.Request(args); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

func (c *Coordinator) Signal(args *SignalArgs, anyErr *error) error {
	if err := c.control.Request(args); err != nil {
		*anyErr = err
		return err
	}

	return nil
}

func (c *Coordinator) RegisterNymph(args *RegisterNymphArgs, reply *bool) error {
	if err := c.control.Request(args); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}

func (c *Coordinator) UnregisterNymph(args *UnregisterNymphArgs, reply *bool) error {
	if err := c.control.Request(args); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}
