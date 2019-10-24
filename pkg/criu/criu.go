package criu

import (
	"fmt"
	"net"
	"runtime"

	"google.golang.org/grpc"
	"github.com/checkpoint-restore/go-criu/rpc"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	"github.com/planetA/konk/pkg/util"
)

type EventType int

const (
	ChunkSize int = 1 << 21
)

const (
	PreDump EventType = iota
	PostDump
	PreRestore
	PostRestore
	PreResume
	PostResume
	SetupNamespaces
	PostSetupNamespaces
	NetworkLock
	NetworkUnlock
	Success
	Error
)

type CriuEvent struct {
	Type     EventType
	Response *rpc.CriuResp
}

func isActive(e EventType) bool {
	return e != Error && e != Success
}

func isValid(e EventType) bool {
	return e != Error
}

func Migrate(cont *container.Container, recipient string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ctx, _ := util.NewContext()
	migration, err := newMigrationClient(ctx, recipient, cont)
	if err != nil {
		return err
	}
	go func() {
		select {
		case <-ctx.Done():
			migration.Close()
		}
	}()
	defer func() {
		migration.Close()
	}()

	err = migration.Run(ctx)
	if err != nil {
		return fmt.Errorf("Migration failed: %v", err)
	}

	return nil
}

// Receive the checkpoint over a created listener
func ReceiveListener(listener net.Listener) (*container.Container, error) {
	grpcServer := grpc.NewServer()
	migrationServer, err := newServer()
	if err != nil {
		return nil, fmt.Errorf("Failed to create migration server: %v", err)
	}
	konk.RegisterMigrationServer(grpcServer, migrationServer)

	cont := make(chan *container.Container, 1)
	go func() {
		cont <- <-migrationServer.Ready
		grpcServer.Stop()
	}()

	grpcServer.Serve(listener)

	return <-cont, nil
}
