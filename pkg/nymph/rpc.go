package nymph

import (
	"syscall"

	"github.com/planetA/konk/pkg/container"
)

// RPC method names

const (
	rpcPrepareReceive = "Nymph.PrepareReceive"
	rpcSend           = "Nymph.Send"
	rpcDelete         = "Nymph.Delete"

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

type DeleteArgs struct {
	ContainerRank container.Rank
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
	Init  bool
}

const (
	rpcImageInfo = "Recipient.ImageInfo"
	rpcLinkInfo  = "Recipient.LinkInfo"
	rpcFileInfo  = "Recipient.FileInfo"
	rpcFileData  = "Recipient.FileData"
	rpcRelaunch  = "Recipient.Relaunch"
)
