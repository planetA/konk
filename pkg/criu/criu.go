package criu

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"time"

	"google.golang.org/grpc"

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

func getSocketPath(pid int) string {
	return fmt.Sprintf("/var/run/criu.service.%v", pid)
}

func getPidfilePath(pid int) string {
	return fmt.Sprintf("/var/run/criu.pidfile.%v", pid)
}

func getImagePath(pid int) string {
	return fmt.Sprintf("%s/pid.%v", util.CriuImageDir, pid)
}

func Dump(pid int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	criu, err := criuFromPid(pid)
	if err != nil {
		return fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}
	defer criu.cleanupService()

	err = criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := criu.nextEvent()
		switch event.Type {
		case Success:
			log.Printf("Dump completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		criu.respond()
	}
}

func Migrate(pid int, recipient string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	criu, err := criuFromPid(pid)
	if err != nil {
		return fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}
	defer criu.cleanupService()

	err = criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := criu.nextEvent()
		switch event.Type {
		case PreDump:
			log.Printf("@pre-dump")
		case PostDump:
			log.Printf("@pre-move")
			if err = criu.moveState(recipient); err != nil {
				return fmt.Errorf("Moving the state failed: %v", err)
			}
			time.Sleep(time.Duration(1) * time.Second)
			log.Printf("@post-move")
		case Success:
			log.Printf("Dump completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		criu.respond()
	}
}

// The recovery server open a port and waits for the dumping server to pass all relevant information
func Receive(portDumper int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", portDumper))
	if err != nil {
		return fmt.Errorf("Failed to open the port: %v", err)
	}

	grpcServer := grpc.NewServer()
	migrationServer, err := newServer()
	if err != nil {
		return fmt.Errorf("Failed to create migration server: %v", err)
	}
	konk.RegisterMigrationServer(grpcServer, migrationServer)
	grpcServer.Serve(listener)

	return nil
}
