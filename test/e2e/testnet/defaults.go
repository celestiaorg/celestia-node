package testnet

import (
	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	rpcPort           = 26658
	p2pPort           = 2121
	prometheusPort    = 9090
	otlpRemotePort    = 4318
	dockerSrcURL      = "ghcr.io/celestiaorg/celestia-node"
	remoteRootDir     = "/home/celestia"
	txsimRootDir      = "/home/celestia"
	celestiaCustomEnv = "CELESTIA_CUSTOM"
)

var DefaultBridgeResources = testnet.Resources{
	MemoryRequest: resource.MustParse("15000Mi"),
	MemoryLimit:   resource.MustParse("16000Mi"),
	CPU:           resource.MustParse("6"),
}

var DefaultFullResources = testnet.Resources{
	MemoryRequest: resource.MustParse("15000Mi"),
	MemoryLimit:   resource.MustParse("16000Mi"),
	CPU:           resource.MustParse("6"),
}

var DefaultLightResources = testnet.Resources{
	MemoryRequest: resource.MustParse("450Mi"),
	MemoryLimit:   resource.MustParse("500Mi"),
	CPU:           resource.MustParse("1"),
}
