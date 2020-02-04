package coordinator

import (
	"sync"

	"github.com/willf/bitset"
)

type NymphSet struct {
	activeIds bitset.BitSet
	set       map[Location]uint
	mutex     sync.Mutex
}

func NewNymphSet() *NymphSet {
	return &NymphSet{
		set:       make(map[Location]uint),
		mutex:     sync.Mutex{},
	}
}

func (n *NymphSet) Add(location Location) uint {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	id, ok := n.activeIds.NextClear(0)
	if ok != true {
		id = n.activeIds.Count()
	}

	n.activeIds.Set(id)
	n.set[location] = id
	return id
}

func (n *NymphSet) Del(location Location) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	id, ok := n.set[location]
	if !ok {
		return false
	}

	n.activeIds.Clear(id)
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
