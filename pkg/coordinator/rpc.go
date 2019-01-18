package coordinator

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcRegisterContainer = "Coordinator.RegisterContainer"
	rpcMigrate           = "Coordinator.Migrate"
	rpcSignal            = "Coordinator.Signal"
)

type RegisterContainerArgs struct {
	Id       container.Id
	Hostname string
}

type MigrateArgs struct {
	Id       container.Id
	DestHost string
}

type SignalArgs struct {
	Signal syscall.Signal
}
