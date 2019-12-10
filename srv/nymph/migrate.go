package nymph

import (
	log "github.com/sirupsen/logrus"

	. "github.com/planetA/konk/pkg/nymph"
)

// Send the checkpoint to the receiving nymph
func (n *Nymph) Send(args *SendArgs, reply *bool) error {
	log.WithFields(log.Fields{
		"host":     args.Host,
		"pre-dump": args.PreDump,
	}).Debug("Received a request to send a checkpoint")

	n.Containers.Mutex.Lock()
	defer n.Containers.Mutex.Unlock()

	container, err := n.Containers.GetUnlocked(args.ContainerRank)
	if err != nil {
		log.WithError(err).WithField("rank", args.ContainerRank).Error("Container not found")
		return err
	}

	checkpoint, err := container.NewCheckpoint()
	if err != nil {
		return err
	}

	if err := checkpoint.Dump(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"path":  checkpoint.PathAbs(),
			"rank":  args.ContainerRank,
		}).Debug("Checkpoint requested")
		return err
	}

	// Establish connection to recipient
	migration, err := NewMigrationDonor(n.RootDir, checkpoint, args.Host)
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

	// Launch remote checkpoint
	err = migration.Relaunch()
	if err != nil {
		log.WithError(err).Debug("Checkpoint send failed")
		return err
	}

	n.Containers.DeleteUnlocked(args.ContainerRank)

	log.Printf("XXX: Need to ensure that container does not exists locally")

	// XXX: Notify the coordinator of the new location

	*reply = true
	return nil
}
