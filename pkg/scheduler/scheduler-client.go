package scheduler

import (
	"fmt"
	"log"
	"net/rpc"

	"github.com/spf13/viper"

	"github.com/planetA/konk/pkg/util"
)

type SchedulerClient struct {
	client *rpc.Client
}

// Create new connection to the scheduler.
// The scheduler is located according to configuration set in viper.
func NewSchedulerClient() (*SchedulerClient, error) {
	hostname := viper.GetString("scheduler.host")
	port := viper.GetInt("scheduler.port")

	rpcClient, err := util.DialRpcServer(hostname, port)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial %v: %v", err)
	}

	return &SchedulerClient{
		client: rpcClient,
	}, nil
}

// Announch where a process that should be managed by the scheduler is running
func (c *SchedulerClient) Announce(rank int, hostname string) error {
	args := &AnnounceArgs{rank, hostname}

	var reply bool
	err := c.client.Call("Scheduler.Announce", args, &reply)

	return err
}

// The container-process tell the scheduler the container id, and address to connect
func (c *SchedulerClient) ContainerAnnounce(rank int, hostname string, port int) error {
	args := &ContainerAnnounceArgs{rank, hostname, port}

	log.Println(args)
	var reply bool
	err := c.client.Call("Scheduler.ContainerAnnounce", args, &reply)

	return err
}

// Request the scheduler to coordinate migration of a process to another node
func (c *SchedulerClient) Migrate(destHost, srcHost string, srcPort int) error {
	args := &MigrateArgs{destHost, srcHost, srcPort}

	log.Println(args)
	var reply bool
	err := c.client.Call("Scheduler.Migrate", args, &reply)

	return err
}

func (c *SchedulerClient) Close() {
	c.client.Close()
}
