package nymph

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive  = "Nymph.PrepareReceive"
	rpcSend            = "Nymph.Send"
	rpcCreateContainer = "Nymph.CreateContainer"
	rpcNotifyProcess   = "Nymph.NotifyProcess"

	rpcSignal = "Nymph.Signal"
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
	Id container.Id
}

type NotifyProcessArgs struct {
	Id container.Id
}

type SignalArgs struct {
	Id     container.Id
	Signal syscall.Signal
}
