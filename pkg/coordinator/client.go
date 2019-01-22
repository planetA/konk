package coordinator

import (
	"fmt"
	"log"
	"net/rpc"
	"syscall"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/util"
)

type Client struct {
	client *rpc.Client
}

// Create new connection to the coordinator.
// The coordinator is located according to configuration set in viper.
func NewClient() (*Client, error) {
	hostname := config.GetString(config.CoordinatorHost)
	port := config.GetInt(config.CoordinatorPort)

	rpcClient, err := util.DialRpcServer(hostname, port)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial %v: %v", err)
	}

	return &Client{
		client: rpcClient,
	}, nil
}

// The container-process tells the coordinator its container id, and address to connect
func (c *Client) RegisterContainer(id container.Id, hostname string) error {
	args := &RegisterContainerArgs{id, hostname}

	log.Println(args)
	var reply bool
	err := c.client.Call(rpcRegisterContainer, args, &reply)

	return err
}

// The container-process tells the coordinator that the container is exiting
func (c *Client) UnregisterContainer(id container.Id) error {
	args := &UnregisterContainerArgs{id}

	log.Println("client coord Unregister", args)
	var reply bool
	err := c.client.Call(rpcUnregisterContainer, args, &reply)

	return err
}

// Request the coordinator to coordinate migration of a process to another node
func (c *Client) Migrate(id container.Id, destHost string) error {
	args := &MigrateArgs{id, destHost}

	log.Println(args)
	var reply bool
	err := c.client.Call(rpcMigrate, args, &reply)

	return err
}

// Send signal to all registered containers via nymphs
//
// XXX: This break single responsibility principle, because migrate and signal interfaces
// are independent, but as long as it is just a single call, it is OK
func (c *Client) Signal(signal syscall.Signal) error {
	args := &SignalArgs{signal}

	log.Println("Sending signal", signal)
	var reply bool
	err := c.client.Call(rpcSignal, args, &reply)

	return err
}

func (c *Client) Close() {
	c.client.Close()
}
