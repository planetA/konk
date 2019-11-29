package nymph

import (
	"github.com/opencontainers/runc/libcontainer"

	log "github.com/sirupsen/logrus"

	. "github.com/planetA/konk/pkg/nymph"
)

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.WithFields(log.Fields{
		"host": args.Host,
	}).Debug("Received a request to send a checkpoint")

	container, err := n.containers.GetUnlocked(args.ContainerRank)
	if err != nil {
		log.WithError(err).WithField("rank", args.ContainerRank).Error("Container not found")
		return err
	}

	err = container.Checkpoint(&libcontainer.CriuOpts{
		ImagesDirectory:   container.CheckpointPath(),
		LeaveRunning:      true,
		TcpEstablished:    true,
		ShellJob:          true,
		FileLocks:         true,
		ManageCgroupsMode: libcontainer.CRIU_CG_MODE_FULL,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"path":  container.CheckpointPath(),
			"rank":  args.ContainerRank,
		}).Debug("Checkpoint requested")
		return err
	}

	// Establish connection to recipient
	migration, err := NewMigrationDonor(container, args.Host)
	if err != nil {
		return err
	}
	defer migration.Close()

	// Send the checkpoint
	err = migration.SendCheckpoint()
	if err != nil {
		log.WithError(err).Debug("Checkpoint send failed")
		return err
	}

	return nil

	// Launch remote checkpoint
	err = migration.Launch()
	if err != nil {
		log.WithError(err).Debug("Checkpoint send failed")
		return err
	}

	log.Printf("XXX: Need to ensure that container does not exists locally")

	// n.forgetContainerRank(args.ContainerRank)
	*reply = true
	return nil
}
