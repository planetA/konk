package scheduler

import (
	"fmt"
	"log"
	"net"
	"net/rpc"

	"github.com/spf13/viper"

	"github.com/ugorji/go/codec"
)

type Scheduler int

type AnnounceArgs struct {
	rank     int
	hostname string
}

func (s *Scheduler) Notify(args *AnnounceArgs, reply *bool) error {
	log.Println("Hello from client!", s)
	*reply = true
	return nil
}

func Run() error {
	var handle codec.MsgpackHandle

	address := fmt.Sprintf(":%v", viper.GetInt("scheduler.port"))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("Could not open a port (%v): %v", address, err)
	}

	log.Printf("Starting scheduler at %v\n", address)

	sched := new(Scheduler)
	rpc.Register(sched)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Failed to open a connection: %v", err)
		}

		rpcCodec := codec.MsgpackSpecRpc.ServerCodec(conn, &handle)
		rpc.ServeCodec(rpcCodec)
	}

	return nil
}
