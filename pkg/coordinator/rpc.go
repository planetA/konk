package coordinator

import (
	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcRegisterContainer = "Coordinator.RegisterContainer"
	rpcMigrate           = "Coordinator.Migrate"
)

type RegisterContainerArgs struct {
	Id       container.Id
	Hostname string
}

type MigrateArgs struct {
	Id       container.Id
	DestHost string
}
