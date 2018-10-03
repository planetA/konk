package node

import (
	"fmt"
	"log"
	"net"

	"github.com/planetA/konk/pkg/util"
)

type Daemon int

type ReceiveArgs struct {
}

// Daemon server code to receive a checkpoint
func (d *Daemon) Receive(args *ReceiveArgs, reply *int) error {
	log.Println("Received a request to receive a checkpoint")

	// Passing port zero will make the kernel arbitrary free port
	listener, err := util.CreateListener(0)
	if err != nil {
		*reply = -1
		return fmt.Errorf("Failed to create a listener")
	}
	*reply = listener.Addr().(*net.TCPAddr).Port

	go func() {
		defer listener.Close()

		log.Println("Receiver is preparing for the migration. Start listening.")
		// err := criu.ReceiveListener(listener)
		// if err != nil {
		// 	return fmt.Errorf("Connection failed: %v", err)
		// }
	}()
	return nil
}

type MigrateArgs struct {
	Host string
	Port int
}

// Daemon server code to send a checkpoint
func (d *Daemon) Migrate(args *MigrateArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)
	*reply = true
	return nil
}
