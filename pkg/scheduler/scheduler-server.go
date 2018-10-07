package scheduler

import (
	"fmt"
	"log"
)

type Location struct {
	Hostname string
	Port     int
	Pid      int
}

type Scheduler struct {
	locationDB map[int]Location
}

type AnnounceArgs struct {
	Rank     int
	Hostname string
}

func (s *Scheduler) Announce(args *AnnounceArgs, reply *bool) error {
	log.Println("Hello from client!", s)
	*reply = true
	return nil
}

type ContainerAnnounceArgs struct {
	Rank     int
	Hostname string
	Port     int
	Pid      int
}

func NewSchedulerServer() *Scheduler {
	return &Scheduler{
		locationDB: make(map[int]Location),
	}
}

func (s *Scheduler) ContainerAnnounce(args *ContainerAnnounceArgs, reply *bool) error {
	s.locationDB[args.Rank] = Location{args.Hostname, args.Port, args.Pid}

	log.Println(s.locationDB)

	*reply = true
	return nil
}

type MigrateArgs struct {
	DestHost string
	Rank     int
}

// Scheduler can receive a migration request from an external entity.
func (s *Scheduler) Migrate(args *MigrateArgs, reply *bool) error {
	log.Printf("Received a request to move rank %v to %v\n", args.Rank, args.DestHost)

	src := s.locationDB[args.Rank]

	if err := Migrate(args.DestHost, src.Hostname, src.Port); err != nil {
		*reply = false
		return fmt.Errorf("Failed to migrate: %v", err)
	}

	*reply = true
	return nil
}
