package testnet

import (
	"time"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	rpcPort           = 26658
	p2pPort           = 2121
	dockerSrcURL      = "ghcr.io/celestiaorg/celestia-node"
	remoteRootDir     = "/home/celestia"
	txsimRootDir      = "/home/celestia"
	celestiaCustomEnv = "CELESTIA_CUSTOM"

	prometheusExporterPort   = 9091
	prometheusScrapeInterval = time.Second * 2
	otlpPort                 = 4317
	otlpRemotePort           = 4318
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
