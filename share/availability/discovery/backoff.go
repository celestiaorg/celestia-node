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

// gcInterval is a default period after which disconnected peers will be removed from cache
const (
	gcInterval = time.Minute
	// connectTimeout is the timeout used for dialing peers and discovering peer addresses.
	connectTimeout = time.Minute * 2
)

var (
	defaultBackoffFactory = backoff.NewFixedBackoff(time.Hour)
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

func (b *backoffConnector) Backoff(p peer.ID) {
	data := b.backoffData(p)
	data.nexttry = time.Now().Add(data.backoff.Delay())

	b.cacheLk.Lock()
	b.cacheData[p] = data
	b.cacheLk.Unlock()
}

func (b *backoffConnector) HasBackoff(p peer.ID) bool {
	b.cacheLk.Lock()
	cache, ok := b.cacheData[p]
	b.cacheLk.Unlock()
	if ok && time.Now().Before(cache.nexttry) {
		return true
	}
	return false
}

// backoffData returns backoffData from the map if it was stored, otherwise it will instantiate
// a new one.
func (b *backoffConnector) backoffData(p peer.ID) backoffData {
	b.cacheLk.Lock()
	defer b.cacheLk.Unlock()

	cache, ok := b.cacheData[p]
	if !ok {
		cache = backoffData{}
		cache.backoff = b.backoff()
		b.cacheData[p] = cache
	}
	return cache
}

// RemoveBackoff removes peer from the backoffCache. It is called when we cancel the attempt to
// connect to a peer after calling Connect.
func (b *backoffConnector) RemoveBackoff(p peer.ID) {
	b.cacheLk.Lock()
	defer b.cacheLk.Unlock()
	delete(b.cacheData, p)
}

// ResetBackoff resets delay time between attempts and adds a delay for the next connection
// attempt to remote peer. It will mostly be called when host receives a notification that remote
// peer was disconnected.
func (b *backoffConnector) ResetBackoff(p peer.ID) {
	cache := b.backoffData(p)
	cache.backoff.Reset()
	b.Backoff(p)
}

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
