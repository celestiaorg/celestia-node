package discovery

import (
	"context"
	"time"

	core "github.com/libp2p/go-libp2p-core/discovery"
	discovery "github.com/libp2p/go-libp2p-discovery"
)

const (
	namespace = "full"
	interval  = time.Second * 10
)

var (
	// waitF calculates time to restart announcing.
	waitF = func(ttl time.Duration) time.Duration {
		return 7 * ttl / 8
	}
)

// Advertise is a utility function that persistently advertises a service through an Advertiser.
func Advertise(ctx context.Context, a core.Advertiser) {
	go func() {
		for {
			ttl, err := a.Advertise(ctx, namespace)
			if err != nil {
				log.Debugf("Error advertising %s: %s", namespace, err.Error())
				if ctx.Err() != nil {
					return
				}

				select {
				case <-time.After(interval):
					continue
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-time.After(waitF(ttl)):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// FindPeers starts peer discovery every 30 seconds until peer cache will not reach peersLimit.
// Can be simplified when https://github.com/libp2p/go-libp2p/pull/1379 will be merged.
func FindPeers(ctx context.Context, d core.Discoverer, n *Notifee) {
	go func() {
		t := time.NewTicker(interval * 3)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				peers, err := discovery.FindPeers(ctx, d, namespace)
				if err != nil {
					continue
				}
				if err = n.HandlePeersFound(namespace, peers); err != nil {
					return
				}
			}
		}
	}()
}
