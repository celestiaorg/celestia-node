package share

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

// backoffConnector wraps a libp2p.Host to establish a connection with peers
// with adding a delay for the next connection attempt.
type backoffConnector struct {
	h       host.Host
	backoff backoff.BackoffFactory

	cacheLk   sync.Mutex
	cacheData map[peer.ID]connectionCacheData
}

// connectionCacheData stores time when next connection attempt with the remote peer.
type connectionCacheData struct {
	nexttry time.Time
	backoff backoff.BackoffStrategy
}

func newBackoffConnector(h host.Host, factory backoff.BackoffFactory) *backoffConnector {
	return &backoffConnector{
		h:         h,
		backoff:   factory,
		cacheData: make(map[peer.ID]connectionCacheData),
	}
}

// Connect puts peer to the backoffCache and tries to establish a connection with it.
func (b *backoffConnector) Connect(ctx context.Context, p peer.AddrInfo) error {
	strategy := b.backoff()
	b.cacheLk.Lock()
	cacheData, ok := b.cacheData[p.ID]
	if ok {
		if time.Now().Before(cacheData.nexttry) {
			b.cacheLk.Unlock()
			return fmt.Errorf("share/discovery: backoff period is not ended for peer=%s", p.ID.String())
		}
		strategy = b.cacheData[p.ID].backoff
	}
	b.cacheData[p.ID] = connectionCacheData{
		time.Now().Add(strategy.Delay()),
		strategy,
	}
	b.cacheLk.Unlock()
	return b.h.Connect(ctx, p)
}

// RestartBackoff resets delay time between attempts and adds a delay for the next connection attempt to remote peer.
// It will mostly be called when host receives a notification that remote peer was disconnected.
func (b *backoffConnector) RestartBackoff(p peer.ID) {
	b.cacheLk.Lock()
	defer b.cacheLk.Unlock()
	cache, ok := b.cacheData[p]
	if !ok {
		return
	}
	cache.backoff.Reset()
	b.cacheData[p] = connectionCacheData{
		time.Now().Add(cache.backoff.Delay()),
		cache.backoff,
	}
}
