package scheduler

import (
	"fmt"
	"net"
	"net/rpc"

	"github.com/spf13/viper"

	"github.com/ugorji/go/codec"
)

type SchedulerClient struct {
	client *rpc.Client
}

// Create new connection to the scheduler.
// The scheduler is located according to configuration set in viper.
func NewSchedulerClient() (*SchedulerClient, error) {
	hostname := viper.GetString("scheduler.host")
	port := viper.GetInt("scheduler.port")

	address := fmt.Sprintf("%v:%v", hostname, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Cannot reach the server (%v): %v", address, err)
	}

	var handle codec.MsgpackHandle
	rpcCodec := codec.MsgpackSpecRpc.ClientCodec(conn, &handle)
	rpcClient := rpc.NewClientWithCodec(rpcCodec)

	return &SchedulerClient{
		client: rpcClient,
	}, nil
}

// Announch where a process that should be managed by the scheduler is running
func (c *SchedulerClient) Announce(rank int, hostname string) (bool, error) {
	args := &AnnounceArgs{rank, hostname}

	var reply bool
	err := c.client.Call("Scheduler.Notify", args, &reply)

	return reply, err
}

func (c *SchedulerClient) Close() {
	c.client.Close()
}
