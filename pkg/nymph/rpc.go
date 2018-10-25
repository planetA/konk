package nymph

import (
	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive  = "Nymph.PrepareReceive"
	rpcSend            = "Nymph.Send"
	rpcCreateContainer = "Nymph.CreateContainer"
	rpcNotifyProcess   = "Nymph.NotifyProcess"
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
