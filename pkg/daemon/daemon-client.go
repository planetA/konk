package daemon

import (
	"net/rpc"

	"github.com/planetA/konk/pkg/util"
)

type DaemonClient struct {
	client *rpc.Client
}

// Create new connection to a node daemon. The address (hostname and port)
// is passed as a parameter. The port is either taken from the configuration
// or was reported preveously by a container process
func NewDaemonClient(hostname string, port int) (*DaemonClient, error) {
	rpcClient, err := util.DialRpcServer(hostname, port)
	if err != nil {
		return nil, err
	}

	return &DaemonClient{
		client: rpcClient,
	}, nil
}

// Connect to a node-daemon and request it to prepare for the checkpoint. The node-daemon
// responds with the port number the sender has to connect to. The caller provides only
// the hostname, the port number of the node-daemon is taken from the configuration.
func (d *DaemonClient) Receive() (int, error) {
	args := &CoReceiverArgs{}

	var reply int
	err := d.client.Call("CoReceiver.Receive", args, &reply)
	if err != nil {
		return -1, err
	}

	return reply, nil
}

func (d *DaemonClient) Migrate(destHost string, destPort int) error {
	args := &CoSenderArgs{destHost, destPort}

	var reply bool
	err := d.client.Call("CoSender.Migrate", args, &reply)
	if err != nil {
		return err
	}

	return nil
}

func (d *DaemonClient) Close() {
	d.client.Close()
}
