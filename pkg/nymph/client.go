// Supporting package for the nymph daemon
package nymph

import (
	"fmt"
	"net/rpc"
	"syscall"

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
	port := config.GetInt(config.NymphPort)
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

// A launcher connects to a local nymph and requests it to create a new container where the actual
// application can run.
func (c *Client) CreateContainer(containerId container.Id, image string) (string, error) {
	args := &CreateContainerArgs{containerId, image}

	var path string
	err := c.client.Call(rpcCreateContainer, args, &path)
	if err != nil {
		return "", fmt.Errorf("RPC call failed: %v", err)
	}

	return path, nil
}

// Tell the nymph that the process has started and the container init process can enter waitpid
func (c *Client) NotifyProcess(containerId container.Id) error {
	args := &NotifyProcessArgs{containerId}

	// Expect no reply
	var reply bool
	if err := c.client.Call(rpcNotifyProcess, args, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Signal(containerId container.Id, signal syscall.Signal) error {
	args := &SignalArgs{containerId, signal}

	var reply bool
	if err := c.client.Call(rpcSignal, args, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Close() {
	c.client.Close()
}
