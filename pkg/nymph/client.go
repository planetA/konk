// Supporting package for the nymph daemon
package nymph

import (
	"os"
	"fmt"
	"net/rpc"
	"syscall"

	log "github.com/sirupsen/logrus"

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

	log.Info("Connected to the coordinator")

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

func (c *Client) Signal(containerId container.Id, signal syscall.Signal) error {
	args := &SignalArgs{containerId, signal}

	log.WithFields(log.Fields{
		"container": containerId,
		"signal": signal}).Trace("Sending signal")
	var reply bool
	if err := c.client.Call(rpcSignal, args, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Run(containerId container.Id, image string, args []string) error {
	runArgs := &RunArgs{containerId, image, args}

	var reply bool
	if err := c.client.Call(rpcRun, runArgs, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Wait(containerId container.Id) (os.ProcessState, error) {
	return os.ProcessState{}, nil
}

func (c *Client) Close() {
	c.client.Close()
}
