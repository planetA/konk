package console

import (
	"fmt"
	"log"
	"strconv"

	"github.com/planetA/konk/pkg/scheduler"
)

func Command(command string, args []string) error {
	log.Printf("Executing a command: %v", command)

	sched, err := scheduler.NewSchedulerClient()
	if err != nil {
		return err
	}
	defer sched.Close()

	switch command {
	case "hi":
		if err := sched.Announce(0, "localhost"); err != nil {
			return fmt.Errorf("Announce failed: %v", err)
		}
		log.Println("Got reply")
	case "migrate":
		if len(args) != 2 {
			return fmt.Errorf("Migrate command usage: migrate <rank> <dest>")
		}

		rank, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("Failed to parse port number (%v): %v", args[2], err)
		}
		destHost := args[1]

		if err := sched.Migrate(destHost, rank); err != nil {
			return fmt.Errorf("Migration failed: %v", err)
		}
	default:
		return fmt.Errorf("Unknown command: %v", command)
	}

	return nil
}
