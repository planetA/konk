package scheduler

import (
	"fmt"
	"log"
)

type Scheduler int

type AnnounceArgs struct {
	Rank     int
	Hostname string
}

func (s *Scheduler) Announce(args *AnnounceArgs, reply *bool) error {
	log.Println("Hello from client!", s)
	*reply = true
	return nil
}

type MigrateArgs struct {
	DestHost string
	SrcHost  string
	SrcPort  int
}

// Scheduler can receive a migration request from an external entity.
func (s *Scheduler) Migrate(args *MigrateArgs, reply *bool) error {
	log.Println("Received a migration request", args.DestHost, args.SrcHost, args.SrcPort)

	if err := Migrate(args.DestHost, args.SrcHost, args.SrcPort); err != nil {
		*reply = false
		log.Println("sth", err)
		return fmt.Errorf("Failed to migrate: %v", err)
	}
	log.Println("thsthsth")
	*reply = true
	return nil
}
