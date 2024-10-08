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
	MemoryRequest: resource.MustParse("1000Mi"),
	MemoryLimit:   resource.MustParse("2000Mi"),
	CPU:           resource.MustParse("300m"),
}

var DefaultFullResources = testnet.Resources{
	MemoryRequest: resource.MustParse("1000Mi"),
	MemoryLimit:   resource.MustParse("2000Mi"),
	CPU:           resource.MustParse("300m"),
}

var DefaultLightResources = testnet.Resources{
	MemoryRequest: resource.MustParse("100Mi"),
	MemoryLimit:   resource.MustParse("200Mi"),
	CPU:           resource.MustParse("100m"),
}
