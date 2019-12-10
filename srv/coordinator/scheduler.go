package coordinator

import (
	"log"
	"math/rand"
	"time"

	"github.com/planetA/konk/pkg/container"
	. "github.com/planetA/konk/pkg/coordinator"
)

type Scheduler struct {
	control *Control
}

func NewScheduler(control *Control) *Scheduler {
	return &Scheduler{
		control: control,
	}
}

func getRanks(locDB LocationDB) []container.Rank {
	ranks := make(map[container.Rank]bool)
	for rank := range locDB.db {
		ranks[rank] = true
	}

	ranksV := make([]container.Rank, len(ranks))
	i := 0
	for rank := range ranks {
		ranksV[i] = rank
		i = i + 1
	}

	return ranksV
}

func (s *Scheduler) Start() {
	ticker := time.NewTicker(10 * time.Second)
	lastLen := 0
	for t := range ticker.C {
		log.Printf("About to reschedule @%v\n", t)
		ranks := getRanks(s.control.locationDB.Dump())
		locs := s.control.nymphSet.GetNymphs()
		log.Printf("RANK: %v LOCATION: %v\n", ranks, locs)

		curLen := len(ranks)
		if !(curLen > 0 && lastLen == curLen) {
			lastLen = curLen
			continue
		}
		lastLen = curLen

		if len(locs) < 2 {
			continue
		}

		// Attempt migration

		// Pick a random container
		targetCont := ranks[rand.Intn(len(ranks))]

		// Figure where it runs
		srcLoc, ok := s.control.locationDB.Get(targetCont)
		if !ok {
			log.Panicf("We've just seen it (%v)! ", targetCont)
		}

		// Remove the source location
		for i := 0; i < len(locs); i++ {
			if locs[i] == srcLoc {
				locs = append(locs[:i], locs[i+1:]...)
				break
			}
		}

		// Pick a target location
		targetLoc := locs[rand.Intn(len(locs))]

		log.Printf("Migrating %v from %v to %v\n", targetCont, srcLoc, targetLoc)
		migrateReq := &MigrateArgs{targetCont, targetLoc.Hostname, false}
		if err := s.control.Request(migrateReq); err != nil {
			log.Println("Failed to migrate: %v", err)
		}
	}
}
