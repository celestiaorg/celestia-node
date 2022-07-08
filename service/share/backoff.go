package share

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

// backoffConnector wraps a host to establish a connection with peers.
type backoffConnector struct {
	h       host.Host
	backoff backoff.BackoffStrategy

	cacheLK      sync.Mutex
	backoffCache map[peer.ID]connectionCacheData
}

// connectionCacheData stores time when next attempt to establish a connection with the remote peer
// will be performed.
type connectionCacheData struct {
	nexttry time.Time
}

func newBackoffConnector(h host.Host, factory backoff.BackoffFactory) *backoffConnector {
	return &backoffConnector{
		h:            h,
		backoff:      factory(),
		backoffCache: make(map[peer.ID]connectionCacheData),
	}
}

// Connect puts peer to the backoffCache and tries to establish a connection with it.
func (b *backoffConnector) Connect(ctx context.Context, p peer.AddrInfo) error {
	b.cacheLK.Lock()
	cacheData, ok := b.backoffCache[p.ID]
	if ok {
		now := time.Now()
		if now.Before(cacheData.nexttry) {
			b.cacheLK.Unlock()
			return errors.New("share: discovery: backoff period is not ended")
		}
	}
	b.backoffCache[p.ID] = connectionCacheData{
		time.Now().Add(b.backoff.Delay()),
	}
	b.cacheLK.Unlock()
	err := b.h.Connect(ctx, p)
	if err != nil {
		return err
	}
	return nil
}

// Reset updates a time of the next attempt.
func (b *backoffConnector) Reset(p peer.ID) {
	b.cacheLK.Lock()
	defer b.cacheLK.Unlock()
	b.backoffCache[p] = connectionCacheData{
		time.Now().Add(b.backoff.Delay()),
	}
}
