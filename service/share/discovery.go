package share

import (
	"context"
	"time"

	core "github.com/libp2p/go-libp2p-core/discovery"
)

const (
	topic    = "full"
	interval = time.Second * 10
)

var (
	// waitF calculates time to restart announcing.
	waitF = func(ttl time.Duration) time.Duration {
		return 7 * ttl / 8
	}
)

// Advertise is a utility function that persistently advertises a service through an Advertiser.
func Advertise(ctx context.Context, a core.Advertiser) {
	for {
		ttl, err := a.Advertise(ctx, topic)
		if err != nil {
			log.Debugf("Error advertising %s: %s", topic, err.Error())
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
}
