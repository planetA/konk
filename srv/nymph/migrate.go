package nymph

import (
	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	. "github.com/planetA/konk/pkg/nymph"
)

func (n *Nymph) sendCheckpoint(checkpoint container.Checkpoint, dest string, launch bool) error {
	// Establish connection to recipient
	migration, err := NewMigrationDonor(n.RootDir, checkpoint, dest)
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

	log.Trace("Checkpoint has been sent")

	if launch {
		// Launch remote checkpoint
		err = migration.Relaunch()
		if err != nil {
			log.WithError(err).Debug("Checkpoint send failed")
			return err
		}

	}

	return nil
}

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.WithFields(log.Fields{
		"host": args.Host,
		"type": args.MigrationType,
	}).Debug("Received a request to send a checkpoint")

	n.Containers.Mutex.Lock()
	defer n.Containers.Mutex.Unlock()

	cont, err := n.Containers.GetUnlocked(args.ContainerRank)
	if err != nil {
		log.WithError(err).WithField("rank", args.ContainerRank).Error("Container not found")
		return err
	}

	var checkpoint container.Checkpoint
	if args.MigrationType == container.PreDump || args.MigrationType == container.WithPreDump {
		// If we need to make pre-dump checkpoint

		// First make checkpoint
		checkpoint, err := cont.NewCheckpoint()
		if err != nil {
			return err
		}

		if err := checkpoint.Dump(true); err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"path":  checkpoint.PathAbs(),
				"rank":  args.ContainerRank,
			}).Debug("Checkpoint requested")
			return err
		}

		// Send without launching
		if err := n.sendCheckpoint(checkpoint, args.Host, false); err != nil {
			return err
		}
	}

	if args.MigrationType == container.Migrate || args.MigrationType == container.WithPreDump {
		// Initiate new checkpoint
		checkpoint, err = cont.NewCheckpoint()
		if err != nil {
			return err
		}

		// Second checkpoint without predump (or first of pre-dump is off)
		if err := checkpoint.Dump(false); err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"path":  checkpoint.PathAbs(),
				"rank":  args.ContainerRank,
			}).Debug("Checkpoint requested")
			return err
		}

		// Now, send a checkpoint and launch it
		if err := n.sendCheckpoint(checkpoint, args.Host, true); err != nil {
			return err
		}

		n.Containers.DeleteUnlocked(args.ContainerRank)
	}

	*reply = true
	return nil
}
