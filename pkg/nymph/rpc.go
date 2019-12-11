package nymph

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive = "Nymph.PrepareReceive"
	rpcSend           = "Nymph.Send"

	rpcSignal = "Nymph.Signal"

	rpcRun = "Nymph.Run"
)

// Container receiving server actually expects no parameters
type ReceiveArgs struct {
}

type SendArgs struct {
	ContainerRank container.Rank
	Host          string
	MigrationType container.MigrationType
}

type CreateContainerArgs struct {
	Rank  container.Rank
	Image string
}

type NotifyProcessArgs struct {
	Rank container.Rank
}

type SignalArgs struct {
	Rank   container.Rank
	Signal syscall.Signal
}

type RunArgs struct {
	Rank  container.Rank
	Image string
	Args  []string
}

const (
	rpcImageInfo = "Recipient.ImageInfo"
	rpcFileInfo  = "Recipient.FileInfo"
	rpcFileData  = "Recipient.FileData"
	rpcRelaunch  = "Recipient.Relaunch"
)
