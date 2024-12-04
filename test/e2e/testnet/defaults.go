package testnet

import (
	"time"

	"github.com/celestiaorg/celestia-app/v3/test/e2e/testnet"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	p2pPortTcp           = 2121
	p2pPortUdp           = 2121
	websocketPort        = 2122
	restPort             = 26659
	rpcPort              = 26658
	dockerSrcURL         = "ghcr.io/celestiaorg/celestia-node"
	remoteRootDir        = "/home/celestia"
	celestiaCustomEnv    = "CELESTIA_CUSTOM"
	celestiaBootstrapEnv = "CELESTIA_BOOTSTRAPPER"

	prometheusExporterPort   = 9091
	prometheusScrapeInterval = time.Second * 2
	otlpPort                 = 4317
	otlpRemotePort           = 4318
)

var DefaultBridgeResources = testnet.Resources{
	MemoryRequest: resource.MustParse("400Mi"),
	MemoryLimit:   resource.MustParse("400Mi"),
	CPU:           resource.MustParse("300m"),
	Volume:        resource.MustParse("1Gi"),
}

var DefaultFullResources = testnet.Resources{
	MemoryRequest: resource.MustParse("400Mi"),
	MemoryLimit:   resource.MustParse("400Mi"),
	CPU:           resource.MustParse("300m"),
	Volume:        resource.MustParse("1Gi"),
}

var DefaultLightResources = testnet.Resources{
	MemoryRequest: resource.MustParse("200Mi"),
	MemoryLimit:   resource.MustParse("200Mi"),
	CPU:           resource.MustParse("150m"),
	Volume:        resource.MustParse("1Gi"),
}
