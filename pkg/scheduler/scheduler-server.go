package scheduler

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

type Scheduler struct {
	locationDB map[int]string
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
}

func NewSchedulerServer() *Scheduler {
	return &Scheduler{
		locationDB: make(map[int]string),
	}
}

func (s *Scheduler) ContainerAnnounce(args *ContainerAnnounceArgs, reply *bool) error {
	s.locationDB[args.Rank] = net.JoinHostPort(args.Hostname, strconv.Itoa(args.Port))

	log.Println(s.locationDB)

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

	*reply = true
	return nil
}
