package coordinator

import (
	"sync"
)

type NymphSet struct {
	set   map[Location]bool
	mutex sync.Mutex
}

func NewNymphSet() *NymphSet {
	return &NymphSet{
		set:   make(map[Location]bool),
		mutex: sync.Mutex{},
	}
}

func (n *NymphSet) Add(location Location) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.set[location] = true
}

func (n *NymphSet) Del(location Location) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	_, ok := n.set[location]
	if !ok {
		return false
	}

	delete(n.set, location)
	return true
}

func (n *NymphSet) GetNymphs() []Location {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	nymphs := make([]Location, len(n.set))
	i := 0
	for nymph := range n.set {
		nymphs[i] = nymph
		i++
	}

	return nymphs
}
