package console

import (
	"log"

	"github.com/planetA/konk/pkg/scheduler"
)

func Command(command string, args []string) error {
	log.Printf("Executing a command: %v", command)

	sched, err := scheduler.NewSchedulerClient()
	if err != nil {
		return err
	}
	defer sched.Close()

	_, err = sched.Announce(0, "localhost")

	log.Println("Got reply")

	return err
}
