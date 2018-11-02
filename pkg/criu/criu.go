package criu

import (
	"fmt"
	"net"
	"runtime"

	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/konk"
	"github.com/planetA/konk/pkg/rpc"
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
	defer func () {
		migration.Close()
	}()

	err = migration.Run(ctx)
	if err != nil {
		return fmt.Errorf("Migration failed: %v", err)
	}

	return nil
}

// The recovery server open a port and waits for the dumping server to pass all relevant information
func Receive(portDumper int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", portDumper))
	if err != nil {
		return fmt.Errorf("Failed to open the port: %v", err)
	}

	if err := ReceiveListener(listener); err != nil {
		return err
	}

	return nil;
}

// Receive the checkpoint over a created listener
func ReceiveListener(listener net.Listener) error {
	grpcServer := grpc.NewServer()
	migrationServer, err := newServer()
	if err != nil {
		return fmt.Errorf("Failed to create migration server: %v", err)
	}
	konk.RegisterMigrationServer(grpcServer, migrationServer)

	go func() {
		<-migrationServer.Ready
		grpcServer.Stop()
	}()

	grpcServer.Serve(listener)

	// XXX: Should be reached when the connection terminates
	// Not reached

	return nil
}
