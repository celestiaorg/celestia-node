package discovery

import (
	discovery "github.com/libp2p/go-libp2p-discovery"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
)

func Null() *Discoverer {
	return NewDiscoverer(NewLimitedSet(0), nil, discovery.NewRoutingDiscovery(routinghelpers.Null{}))
}
