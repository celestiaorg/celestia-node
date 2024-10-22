package store

import (
	"sync"

	"github.com/celestiaorg/celestia-node/share"
)

// TODO: move to utils
type striplock struct {
	heights    []*sync.RWMutex
	datahashes []*sync.RWMutex
}

type multiLock struct {
	mu []*sync.RWMutex
}

func newStripLock(size int) *striplock {
	heights := make([]*sync.RWMutex, size)
	datahashes := make([]*sync.RWMutex, size)
	for i := 0; i < size; i++ {
		heights[i] = &sync.RWMutex{}
		datahashes[i] = &sync.RWMutex{}
	}
	return &striplock{heights, datahashes}
}

func (l *striplock) byHeight(height uint64) *sync.RWMutex {
	lkIdx := height % uint64(len(l.heights))
	return l.heights[lkIdx]
}

func (l *striplock) byHash(datahash share.DataHash) *sync.RWMutex {
	// Use the last 2 bytes of the hash as key to distribute the locks
	last := uint16(datahash[len(datahash)-1]) | uint16(datahash[len(datahash)-2])<<8
	lkIdx := last % uint16(len(l.datahashes))
	return l.datahashes[lkIdx]
}

func (l *striplock) byHashAndHeight(datahash share.DataHash, height uint64) *multiLock {
	return &multiLock{[]*sync.RWMutex{l.byHash(datahash), l.byHeight(height)}}
}

func (m *multiLock) lock() {
	for _, lk := range m.mu {
		lk.Lock()
	}
}

func (m *multiLock) unlock() {
	for _, lk := range m.mu {
		lk.Unlock()
	}
}
