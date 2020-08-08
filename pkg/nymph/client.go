// Supporting package for the nymph daemon
package nymph

import (
	"fmt"
	"net/rpc"
	"os"
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

// Send the checkpoint to the server at given host and port. The receiver is a nymph, but the
// port is supposed to be not the default nymph port.
func (c *Client) Send(containerRank container.Rank, destHost string, migrationType container.MigrationType) error {
	args := &SendArgs{containerRank, destHost, migrationType}

	var reply bool
	err := c.client.Call(rpcSend, args, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Delete(containerRank container.Rank) error {
	args := &DeleteArgs{containerRank}

	var reply bool
	err := c.client.Call(rpcDelete, args, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Signal(containerRank container.Rank, signal syscall.Signal) error {
	args := &SignalArgs{containerRank, signal}

	log.WithFields(log.Fields{
		"container": containerRank,
		"signal":    signal}).Trace("Sending signal")
	var reply bool
	if err := c.client.Call(rpcSignal, args, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Run(runArgs *RunArgs) error {
	var reply bool
	if err := c.client.Call(rpcRun, runArgs, &reply); err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	return nil
}

func (c *Client) Wait(containerRank container.Rank) (os.ProcessState, error) {
	return os.ProcessState{}, nil
}

func (c *Client) Close() {
	c.client.Close()
}
