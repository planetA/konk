package nymph

import (
	"os"
	"syscall"
	"time"

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
	rpcHello     = "Recipient.Hello"
	rpcImageInfo = "Recipient.ImageInfo"
	rpcFileInfo  = "Recipient.FileInfo"
)

type HelloArgs struct {
	Say string
}

type ImageInfoArgs struct {
	Rank container.Rank
	ID   string
}

type FileInfoArgs struct {
	Filename string
	Size     int64
	Mode     os.FileMode
	ModTime  time.Time
}
