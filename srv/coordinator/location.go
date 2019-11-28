package coordinator

import (
	"fmt"
	"sync"

	"github.com/planetA/konk/pkg/container"
)

type Location struct {
	Hostname string
}

type LocationDB struct {
	db    map[container.Rank]Location
	mutex sync.Mutex
}

func NewLocationDB() *LocationDB {
	return &LocationDB{
		db:    make(map[container.Rank]Location),
		mutex: sync.Mutex{},
	}
}

func (l *LocationDB) Get(rank container.Rank) (Location, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	loc, ok := l.db[rank]
	return loc, ok
}

func (l *LocationDB) Set(rank container.Rank, location Location) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.db[rank] = location
}

func (l *LocationDB) Unset(rank container.Rank, oldLocation Location) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	curHost, ok := l.db[rank]
	if ok && curHost == oldLocation {
		delete(l.db, rank)
	} else if !ok {
		return fmt.Errorf("Container %v was not registered", rank)
	} else if curHost != oldLocation {
		return fmt.Errorf("Request for deleting %v@%v came from %v", rank, curHost, oldLocation)
	}

	return nil
}

func (l *LocationDB) Dump() LocationDB {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return *l
}
