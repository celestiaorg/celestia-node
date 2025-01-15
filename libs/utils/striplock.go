package utils

import (
	"sync"

	"github.com/celestiaorg/celestia-node/share"
)

// StripLock provides a way to lock operations by height and data hash.
// It uses a fixed number of mutexes to avoid excessive memory allocation
// while still providing good concurrency characteristics.
type StripLock struct {
	heights    []*sync.RWMutex
	datahashes []*sync.RWMutex
}

// MultiLock represents multiple locks that need to be acquired together.
type MultiLock struct {
	mu []*sync.RWMutex
}

// NewStripLock creates a new StripLock with the specified size of mutex pools.
func NewStripLock(size int) *StripLock {
	heights := make([]*sync.RWMutex, size)
	datahashes := make([]*sync.RWMutex, size)
	for i := 0; i < size; i++ {
		heights[i] = &sync.RWMutex{}
		datahashes[i] = &sync.RWMutex{}
	}
	return &StripLock{heights, datahashes}
}

// ByHeight returns a mutex for the given height.
func (l *StripLock) ByHeight(height uint64) *sync.RWMutex {
	lkIdx := height % uint64(len(l.heights))
	return l.heights[lkIdx]
}

// ByHash returns a mutex for the given data hash.
func (l *StripLock) ByHash(datahash share.DataHash) *sync.RWMutex {
	// Use the last 2 bytes of the hash as key to distribute the locks
	last := uint16(datahash[len(datahash)-1]) | uint16(datahash[len(datahash)-2])<<8
	lkIdx := last % uint16(len(l.datahashes))
	return l.datahashes[lkIdx]
}

// ByHashAndHeight returns a MultiLock for both the hash and height.
func (l *StripLock) ByHashAndHeight(datahash share.DataHash, height uint64) *MultiLock {
	return &MultiLock{[]*sync.RWMutex{l.ByHash(datahash), l.ByHeight(height)}}
}

// Lock acquires all contained locks.
func (m *MultiLock) Lock() {
	for _, lk := range m.mu {
		lk.Lock()
	}
}

// Unlock releases all contained locks.
func (m *MultiLock) Unlock() {
	for _, lk := range m.mu {
		lk.Unlock()
	}
} 
