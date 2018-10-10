package nymph

import (
	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive = "Nymph.PrepareReceive"
	rpcSend           = "Nymph.Send"
	rpcRegister       = "Nymph.Register"
)

// Container receiving server actually expects no parameters
type ReceiveArgs struct {
}

type SendArgs struct {
	ContainerId container.Id
	Host        string
	Port        int
}

type RegisterArgs struct {
	Id  container.Id
	Pid int
}
