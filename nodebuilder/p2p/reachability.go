package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

const reachabilityCheckTick = 10 * time.Second

func reachabilityCheck(ctx context.Context, host host.Host) {
	getter, ok := host.(autoNatGetter)
	if !ok {
		panic("host does not implement autoNatGetter")
	}
	autoNAT := getter.GetAutoNat()

	go func() {
		timer := time.NewTimer(reachabilityCheckTick)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				reachability := autoNAT.Status()
				if reachability == network.ReachabilityPublic {
					return
				}

				log.Error("Host is not reachable from the public network")
			case <-ctx.Done():
				return
			}
		}
	}()
}
