package nymph

import (
	"fmt"
	"net"
	"os"

	"github.com/opencontainers/runc/libcontainer"

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
		cont, err := ReceiveListener(listener)
		if err != nil {
			log.Panicf("Connection failed: %v", err)
		}

		panic("Unimplemented")
		// n.rememberContainer(cont)

		hostname, err := os.Hostname()
		if err != nil {
			log.Panicf("Failed to get hostname: %v", err)
		}

		err = n.coordinatorClient.RegisterContainer(cont.Rank(), hostname)
		if err != nil {
			log.Panicf("Registering at the coordinator failed: %v", err)
		}
	}()

	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.Println("Received a request to send a checkpoint to ", args.Host, args.Port)

	container, err := n.containers.GetUnlocked(args.ContainerRank)
	if err != nil {
		log.WithError(err).WithField("rank", args.ContainerRank).Error("Container not found")
		return err
	}

	err = container.Checkpoint(&libcontainer.CriuOpts{
		ImagesDirectory: n.imagesPath(),
		WorkDirectory:   n.criuPath(),
		// LeaveRunning:      true,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_FULL,
	})
	log.WithError(err).Debug("Checkpoint requeted")

	// address := net.JoinHostPort(args.Host, strconv.Itoa(args.Port))
	// err = Migrate(container, address)
	// if err != nil {
	// 	*reply = false
	// 	return err
	// }

	// n.forgetContainerRank(args.ContainerRank)
	*reply = true
	return nil
}
