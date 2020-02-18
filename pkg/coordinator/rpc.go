package coordinator

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcAllocateHost   = "Coordinator.AllocateHost"

	rpcRegisterContainer   = "Coordinator.RegisterContainer"
	rpcUnregisterContainer = "Coordinator.UnregisterContainer"
	rpcMigrate             = "Coordinator.Migrate"
	rpcSignal              = "Coordinator.Signal"

	rpcRegisterNymph   = "Coordinator.RegisterNymph"
	rpcUnregisterNymph = "Coordinator.UnregisterNymph"
)

type AllocateHostArgs struct {
	Rank container.Rank
}

type RegisterContainerArgs struct {
	Rank     container.Rank
	Hostname string
}

type UnregisterContainerArgs struct {
	Rank     container.Rank
	Hostname string
}

type MigrateArgs struct {
	Rank          container.Rank
	DestHost      string
	MigrationType container.MigrationType
}

type SignalArgs struct {
	Signal syscall.Signal
}

type RegisterNymphArgs struct {
	Hostname string
}

type UnregisterNymphArgs struct {
	Hostname string
}
