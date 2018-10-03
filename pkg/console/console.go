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
		if len(args) != 3 {
			return fmt.Errorf("Migrate command usage: migrate <dest> <src> <src-port>")
		}

		destHost := args[0]
		srcHost := args[1]
		srcPort, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("Failed to parse port number (%v): %v", args[2], err)
		}

		if err := sched.Migrate(destHost, srcHost, srcPort); err != nil {
			return fmt.Errorf("Migration failed: %v", err)
		}
	default:
		return fmt.Errorf("Unknown command: %v", command)
	}

	return nil
}
