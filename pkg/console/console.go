package console

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
)

func Command(command string, args []string) error {
	log.Printf("Executing a command: %v", command)

	coord, err := coordinator.NewClient()
	if err != nil {
		return err
	}
	defer coord.Close()

	switch command {
	case "migrate":
		if len(args) != 2 {
			return fmt.Errorf("Migrate command usage: migrate <rank> <dest>")
		}

		rank, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("Failed to parse rank (%v): %v", args[0], err)
		}
		destHost := args[1]

		log.WithFields(log.Fields{
			"rank": rank,
			"dest": destHost,
		}).Debug("Requesting migration")
		if err := coord.Migrate(container.Rank(rank), destHost); err != nil {
			return fmt.Errorf("Migration failed: %v", err)
		}
	default:
		return fmt.Errorf("Unknown command: %v", command)
	}

	return nil
}
