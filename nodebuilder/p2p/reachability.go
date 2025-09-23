package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
)

const reachabilityCheckTick = 10 * time.Second

func reachabilityCheck(ctx context.Context, host HostBase) {
	log.Info("reachabilityCheck", "host", host)
	getter, ok := host.(autoNatGetter)
	if !ok {
		panic("host does not implement autoNatGetter")
	}

	autoNAT := getter.GetAutoNat()
	if autoNAT == nil {
		log.Error("autoNAT is nil on host")
		return
	}
	log.Info("autoNAT", "autoNAT", autoNAT)

	go func() {
		ticker := time.NewTicker(reachabilityCheckTick)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reachability := autoNAT.Status()
				log.Info("reachability", "reachability", reachability, network.ReachabilityPublic)
				if reachability == network.ReachabilityPublic {
					return
				}

				log.Warn("Host is not reachable from the public network!")
				return

				// log.Error(`Host is not reachable from the public network!
				//See https://docs.celestia.org/how-to-guides/celestia-node-troubleshooting#ports
				//`)
			case <-ctx.Done():
				return
			}
		}
	}()
}
