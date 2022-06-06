package discovery

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

// PeerCache holds all discovered and available peers
type PeerCache struct {
	mu    sync.RWMutex
	cache map[peer.ID]bool
}

// NewPeerCache constructs PeerCache
func NewPeerCache() *PeerCache {
	return &PeerCache{
		cache: make(map[peer.ID]bool),
	}
}

// Add adds connected peer to cache
func (cache *PeerCache) Add(id peer.ID) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.cache[id] = true
}

// Remove removes peer from cache when it goes offline
func (cache *PeerCache) Remove(id peer.ID) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.cache, id)
}

// Has checks the availability of the peer
func (cache *PeerCache) Has(id peer.ID) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	_, has := cache.cache[id]
	return has
}

func (cache *PeerCache) Size() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return len(cache.cache)
}
