package store

import (
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

// stripLock is deprecated, use utils.StripLock instead
type striplock = utils.StripLock

// multiLock is deprecated, use utils.MultiLock instead
type multiLock = utils.MultiLock

// newStripLock creates a new StripLock with the specified number of mutexes.
// Deprecated: use utils.NewStripLock instead
func newStripLock(size int) *striplock {
	return utils.NewStripLock(size)
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
