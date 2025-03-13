package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
)

const reachabilityCheckTick = 10 * time.Second

func reachabilityCheck(ctx context.Context, host HostBase) {
	getter, ok := host.(autoNatGetter)
	if !ok {
		panic("host does not implement autoNatGetter")
	}
	autoNAT := getter.GetAutoNat()

	go func() {
		ticker := time.NewTicker(reachabilityCheckTick)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if autoNAT == nil {
					log.Error("autoNAT is nil on host")
					continue
				}

				reachability := autoNAT.Status()
				if reachability == network.ReachabilityPublic {
					return
				}

				log.Error(`Host is not reachable from the public network!
See https://docs.celestia.org/how-to-guides/celestia-node-troubleshooting#ports
`)
			case <-ctx.Done():
				return
			}
		}
	}()
}
