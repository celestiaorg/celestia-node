package discovery

import (
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

func Null() *Discoverer {
	return NewDiscoverer(NewLimitedSet(0), nil, routing.NewRoutingDiscovery(routinghelpers.Null{}))
}
