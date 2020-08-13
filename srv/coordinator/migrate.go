package coordinator

import (
	"fmt"
	"syscall"
	"time"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
	log "github.com/sirupsen/logrus"
)

// The scheduler coordinates the migration process between container process and a node-daemon.
// First, the scheduler connects to the destination nymph and asks to prepare for receiving the
// checkpoint. The nymph returns the port number that should be used specifically for transferring
// this particular checkpoint. Then, the coordinator contacts the source nymph, tells it the
// destination hostname and port number, and asks to send the checkpoint.
func Migrate(containerRank container.Rank, srcHost, destHost string, migrationType container.MigrationType) error {
	if srcHost == destHost {
		return fmt.Errorf("The container is already at the destination")
	}

	// Tell the nymph to migrate the container to another nymph
	donorClient, err := nymph.NewClient(srcHost)
	if err != nil {
		return fmt.Errorf("Failed to reach nymph: %v", err)
	}
	defer donorClient.Close()

	log.WithFields(log.Fields{
		"rank": containerRank,
		"src":  srcHost,
		"dst":  destHost,
		"type": migrationType,
	}).Trace("Requesting migration")

	start := time.Now()
	err = donorClient.Send(containerRank, destHost, migrationType)
	if err != nil {
		return fmt.Errorf("Container did not migrate: %v", err)
	}

	elapsed := time.Since(start)
	log.WithField("elapsed", elapsed).Info("Migration finished successfully")

	return nil
}

func Delete(containerRank container.Rank, destHost string) error {
	// Tell the nymph to migrate the container to another nymph
	client, err := nymph.NewClient(destHost)
	if err != nil {
		return fmt.Errorf("Failed to reach nymph: %v", err)
	}
	defer client.Close()

	log.WithFields(log.Fields{
		"rank": containerRank,
		"host": destHost,
	}).Trace("Deleting container")

	err = client.Delete(containerRank)
	if err != nil {
		return fmt.Errorf("Failed to delete container: %v", err)
	}

	return nil
}

func Signal(containerRank container.Rank, host string, signal syscall.Signal) error {
	client, err := nymph.NewClient(host)
	if err != nil {
		return fmt.Errorf("Failed to connect to the nymph %v: %v", host, err)
	}
	defer client.Close()

	err = client.Signal(containerRank, signal)
	if err != nil {
		return fmt.Errorf("Sending signal to the coordinator failed: %v", err)
	}

	return nil
}
