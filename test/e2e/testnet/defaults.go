package testnet

import "github.com/celestiaorg/celestia-app/v2/test/e2e/testnet"

var DefaultBridgeResources = testnet.Resources{
	MemoryRequest: "1000Mi",
	MemoryLimit:   "2000Mi",
	CPU:           "300m",
}

var DefaultFullResources = testnet.Resources{
	MemoryRequest: "1000Mi",
	MemoryLimit:   "2000Mi",
	CPU:           "300m",
}

var DefaultLightResources = testnet.Resources{
	MemoryRequest: "100Mi",
	MemoryLimit:   "200Mi",
	CPU:           "100m",
}
