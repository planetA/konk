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
	db    map[container.Id]Location
	mutex sync.Mutex
}

func NewLocationDB() *LocationDB {
	return &LocationDB{
		db:    make(map[container.Id]Location),
		mutex: sync.Mutex{},
	}
}

func (l *LocationDB) Get(id container.Id) (Location, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	loc, ok := l.db[id]
	return loc, ok
}

func (l *LocationDB) Set(id container.Id, location Location) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.db[id] = location
}

func (l *LocationDB) Unset(id container.Id, oldLocation Location) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	curHost, ok := l.db[id]
	if ok && curHost == oldLocation {
		delete(l.db, id)
	} else if !ok {
		return fmt.Errorf("Container %v was not registered", id)
	} else if curHost != oldLocation {
		return fmt.Errorf("Request for deleting %v@%v came from %v", id, curHost, oldLocation)
	}

	return nil
}

func (l *LocationDB) Dump() LocationDB {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return *l
}
