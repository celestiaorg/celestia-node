package p2p

import (
	"github.com/libp2p/go-libp2p-core/network"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
)

func resourceManager() (network.ResourceManager, error) {
	return rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()))
}
