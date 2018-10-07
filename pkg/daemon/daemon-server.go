package daemon

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/util"
)

// Type for container receiving server
type CoReceiver struct {
	_ int
}

// Container receiving server actually expects no parameters
type CoReceiverArgs struct {
}

// Container receiving server has only one method
func (r *CoReceiver) Receive(args *CoReceiverArgs, reply *int) error {
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
		err := criu.ReceiveListener(listener)
		if err != nil {
			log.Panicf("Connection failed: %v", err)
		}
	}()
	return nil
}

// Type for container sending server
type CoSender struct {
	Pid int
}

type CoSenderArgs struct {
	Host string
	Port int
}

func NewCoSender(pid int) *CoSender {
	return &CoSender{
		Pid: pid,
	}
}

// Container sending server also has only one method
func (s *CoSender) Migrate(args *CoSenderArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	if err := criu.Migrate(s.Pid, address); err != nil {
		*reply = false
		return err
	}

	*reply = true
	return nil
}
