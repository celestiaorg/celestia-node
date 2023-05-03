package discovery

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

const (
	// gcInterval is a default period after which disconnected peers will be removed from cache
	gcInterval = time.Minute
	// connectTimeout is the timeout used for dialing peers and discovering peer addresses.
	connectTimeout = time.Minute * 2
)

var (
	defaultBackoffFactory = backoff.NewFixedBackoff(time.Minute * 10)
	errBackoffNotEnded    = errors.New("share/discovery: backoff period has not ended")
)

// backoffConnector wraps a libp2p.Host to establish a connection with peers
// with adding a delay for the next connection attempt.
type backoffConnector struct {
	h       host.Host
	backoff backoff.BackoffFactory

	cacheLk   sync.Mutex
	cacheData map[peer.ID]backoffData
}

// backoffData stores time when next connection attempt with the remote peer.
type backoffData struct {
	nexttry time.Time
	backoff backoff.BackoffStrategy
}

func newBackoffConnector(h host.Host, factory backoff.BackoffFactory) *backoffConnector {
	return &backoffConnector{
		h:         h,
		backoff:   factory,
		cacheData: make(map[peer.ID]backoffData),
	}
}

// Connect puts peer to the backoffCache and tries to establish a connection with it.
func (b *backoffConnector) Connect(ctx context.Context, p peer.AddrInfo) error {
	if b.HasBackoff(p.ID) {
		return errBackoffNotEnded
	}

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	err := b.h.Connect(ctx, p)
	// we don't want to add backoff when the context is canceled.
	if !errors.Is(err, context.Canceled) {
		b.Backoff(p.ID)
	}
	return err
}

// Backoff adds or extends backoff delay for the peer.
func (b *backoffConnector) Backoff(p peer.ID) {
	b.cacheLk.Lock()
	defer b.cacheLk.Unlock()

	data, ok := b.cacheData[p]
	if !ok {
		data = backoffData{}
		data.backoff = b.backoff()
		b.cacheData[p] = data
	}

	data.nexttry = time.Now().Add(data.backoff.Delay())
	b.cacheData[p] = data
}

// HasBackoff checks if peer is in backoff.
func (b *backoffConnector) HasBackoff(p peer.ID) bool {
	b.cacheLk.Lock()
	cache, ok := b.cacheData[p]
	b.cacheLk.Unlock()
	return ok && time.Now().Before(cache.nexttry)
}

// GC is a perpetual GCing loop.
func (b *backoffConnector) GC(ctx context.Context) {
	ticker := time.NewTicker(gcInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.cacheLk.Lock()
			for id, cache := range b.cacheData {
				if cache.nexttry.Before(time.Now()) {
					delete(b.cacheData, id)
				}
			}
			b.cacheLk.Unlock()
		}
	}
}
