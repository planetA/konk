package coordinator

import (
	"log"
	"time"
	"math/rand"

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

func getIdsNodes(locDB LocationDB) ([]container.Id, []Location) {
	ids := make(map[container.Id]bool)
	locs := make(map[Location]bool)
	for id, loc := range locDB.db {
		ids[id] = true
		locs[loc] = true
	}

	idsV := make([]container.Id, len(ids))
	i := 0
	for id := range ids {
		idsV[i] = id
		i = i + 1
	}

	locsV := make([]Location, len(locs))
	i = 0
	for loc := range locs {
		locsV[i] = loc
		i = i + 1
	}

	return idsV, locsV
}

func (s *Scheduler) Start() {
	ticker := time.NewTicker(10 * time.Second)
	lastLen := 0
	for t := range ticker.C {
		log.Printf("About to reschedule @%v\n", t)
		ids, locs := getIdsNodes(s.control.locationDB.Dump())
		log.Printf("ID: %v LOCATION: %v\n", ids, locs)

		curLen := len(ids)
		if ! (curLen > 0 && lastLen == curLen) {
			lastLen = curLen
			continue
		}
		lastLen = curLen

		// Attempt migration

		// Pick a random container
		targetCont := ids[rand.Intn(len(ids))]

		// Figure where it runs
		srcLoc, ok := s.control.locationDB.Get(targetCont)
		if ! ok {
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
		migrateReq := &MigrateArgs{targetCont, targetLoc.Hostname}
		if err := s.control.Request(migrateReq); err != nil {
			log.Println("Failed to migrate: %v", err)
		}
	}
}
