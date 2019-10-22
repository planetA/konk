package nymph

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive  = "Nymph.PrepareReceive"
	rpcSend            = "Nymph.Send"

	rpcSignal = "Nymph.Signal"

	rpcRun = "Nymph.Run"
)

// Container receiving server actually expects no parameters
type ReceiveArgs struct {
}

type SendArgs struct {
	ContainerId container.Id
	Host        string
	Port        int
}

type CreateContainerArgs struct {
	Id    container.Id
	Image string
}

type NotifyProcessArgs struct {
	Id container.Id
}

type SignalArgs struct {
	Id     container.Id
	Signal syscall.Signal
}

type RunArgs struct {
	Id    container.Id
	Image string
	Args  []string
}
