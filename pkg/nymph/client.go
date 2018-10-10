// Supporting package for the nymph daemon
package nymph

import (
	"net/rpc"

	"github.com/spf13/viper"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/util"
)

// Client to connect to the nymph daemon
type Client struct {
	client *rpc.Client
}

// Create new connection to a node daemon. The address (hostname and port)
// is passed as a parameter. The port is either taken from the configuration
// or was reported preveously by a container process
func NewClient(hostname string) (*Client, error) {
	port := viper.GetInt(config.ViperNymphPort)
	rpcClient, err := util.DialRpcServer(hostname, port)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: rpcClient,
	}, nil
}

// Connect to a node-daemon and request it to prepare for the checkpoint. The node-daemon
// responds with the port number the sender has to connect to. The caller provides only
// the hostname, the port number of the node-daemon is taken from the configuration.
func (c *Client) PrepareReceive() (int, error) {
	args := &ReceiveArgs{}

	var reply int
	err := c.client.Call(rpcPrepareReceive, args, &reply)
	if err != nil {
		return -1, err
	}

	return reply, nil
}

// Send the checkpoint to the server at given host and port. The receiver is a nymph, but the
// port is supposed to be not the default nymph port.
func (c *Client) Send(containerId container.Id, destHost string, destPort int) error {
	args := &SendArgs{containerId, destHost, destPort}

	var reply bool
	err := c.client.Call(rpcSend, args, &reply)
	if err != nil {
		return err
	}

	return nil
}

// A container-process connects to a local nymph and reports the pid of the root process of
// the container
func (c *Client) Register(containerId container.Id, pid int) error {
	args := &RegisterArgs{containerId, pid}

	var reply bool
	err := c.client.Call(rpcRegister, args, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Close() {
	c.client.Close()
}
