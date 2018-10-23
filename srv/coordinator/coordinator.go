package coordinator

import (
	"fmt"
	"net/rpc"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/util"
)

func Run() error {
	listener, err := util.CreateListener(config.GetInt(config.CoordinatorPort))
	if err != nil {
		return fmt.Errorf("Can't launch coordinator server: %v", err)
	}
	defer listener.Close()

	coord := NewCoordinator()
	rpc.Register(coord)

	if err := util.ServerLoop(listener); err != nil {
		return fmt.Errorf("Coordinator server failed: %v", err)
	}

	return nil
}
