package nymph

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	. "github.com/planetA/konk/pkg/nymph"
)

func (n *Nymph) sendCheckpoint(cont *container.Container, checkpoint container.Checkpoint, dest string, launch bool) error {
	// Establish connection to recipient
	migration, err := NewMigrationDonor(n.RootDir, checkpoint, dest)
	if err != nil {
		return err
	}
	defer migration.Close()

	if checkpoint.IsPageServer() {
		if err := migration.StartPageServer(checkpoint.PathAbs()); err != nil {
			log.WithError(err).Error("Migration server did not start")
			return err
		}
	}

	start := time.Now()
	// Second checkpoint without predump (or first of pre-dump is off)
	if err := checkpoint.Dump(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"path":  checkpoint.PathAbs(),
			"rank":  checkpoint.Rank(),
		}).Debug("Checkpoint requested")
		return err
	}
	elapsed := time.Since(start)
	log.WithField("elapsed", elapsed).Info("Dump finished successfully")

	// Send the checkpoint
	start = time.Now()
	err = migration.SendCheckpoint(cont)
	if err != nil {
		log.WithError(err).Debug("Checkpoint send failed")
		return err
	}
	log.WithField("elapsed", time.Since(start)).Info("Checkpoint has been sent")

	start = time.Now()
	if launch {
		// Launch remote checkpoint
		err = migration.Relaunch()
		if err != nil {
			log.WithError(err).Debug("Checkpoint send failed")
			return err
		}

	}
	log.WithField("elapsed", time.Since(start)).Info("Relaunch finished")

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

	log.WithField("page-server", args.PageServer).Info("Request to dump")

	var checkpoint container.Checkpoint = nil
	if args.MigrationType == container.PreDump || args.MigrationType == container.WithPreDump {
		// If we need to make pre-dump checkpoint

		// First make checkpoint, that has no parent
		pageServer := ""
		if args.PageServer {
			pageServer = args.Host
		}
		checkpoint, err = cont.NewCheckpoint(&container.CheckpointArgs{
			Parent:     nil,
			PreDump:    true,
			PageServer: pageServer,
		})
		if err != nil {
			return err
		}

		// Send without launching
		if err := n.sendCheckpoint(nil, checkpoint, args.Host, false); err != nil {
			return err
		}
	}

	// if args.MigrationType == container.WithPreDump {
	// 	time.Sleep(2 * time.Second)
	// }

	if args.MigrationType == container.Migrate || args.MigrationType == container.WithPreDump {
		// Initiate new checkpoint. If there was parent, we use it.
		pageServer := ""
		if args.PageServer {
			pageServer = args.Host
		}
		checkpoint, err = cont.NewCheckpoint(&container.CheckpointArgs{
			Parent:     checkpoint,
			PreDump:    false,
			PageServer: pageServer,
		})
		if err != nil {
			return err
		}

		// Now, send a checkpoint and launch it
		if err := n.sendCheckpoint(cont, checkpoint, args.Host, true); err != nil {
			return err
		}

		n.Containers.DeleteUnlocked(args.ContainerRank)
	}

	*reply = true
	return nil
}
