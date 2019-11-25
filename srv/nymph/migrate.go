package nymph

import (
	"fmt"
	"net"
	"os"

	"github.com/opencontainers/runc/libcontainer"

	"github.com/planetA/konk/pkg/criu"
	"github.com/planetA/konk/pkg/util"
	log "github.com/sirupsen/logrus"

	. "github.com/planetA/konk/pkg/nymph"
)

// Container receiving server has only one method
func (n *Nymph) PrepareReceive(args *ReceiveArgs, reply *int) error {
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
		cont, err := criu.ReceiveListener(listener)
		if err != nil {
			log.Panicf("Connection failed: %v", err)
		}

		panic("Unimplemented")
		// n.rememberContainer(cont)

		hostname, err := os.Hostname()
		if err != nil {
			log.Panicf("Failed to get hostname: %v", err)
		}

		err = n.coordinatorClient.RegisterContainer(cont.Id, hostname)
		if err != nil {
			log.Panicf("Registering at the coordinator failed: %v", err)
		}
	}()

	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	// address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	container, _ := n.containers.GetUnlocked(args.ContainerId)

	err := container.Checkpoint(&libcontainer.CriuOpts{
		ImagesDirectory:   n.imagesPath(),
		WorkDirectory:     n.criuPath(),
		LeaveRunning:      true,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_FULL,
	})
	log.WithError(err).Debug("Checkpoint requeted")

	// if err := criu.Migrate(container, address); err != nil {
	// 	*reply = false
	// 	return err
	// }

	// n.forgetContainerId(args.ContainerId)
	*reply = true
	return nil
}
