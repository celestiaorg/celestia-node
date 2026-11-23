package p2p

import (
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

func resourceManager(params resourceManagerParams) (network.ResourceManager, error) {
	return rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(params.Limits), params.Opts...)
}

// bridgeResources returns resource manager limits for bridge nodes.
// instead of infinite limits, it uses auto-scaled defaults derived from available
// system resources, augmented with explicit service-level and per-protocol limits
// for the shrex protocol to prevent resource exhaustion attacks.
func bridgeResources(network Network) rcmgr.ConcreteLimitConfig {
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	shrex.SetResourceLimits(&limits, network.String())
	return limits.AutoScale()
}

func autoscaleResources() rcmgr.ConcreteLimitConfig {
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	return limits.AutoScale()
}

func traceReporter() rcmgr.Option {
	str, err := rcmgr.NewStatsTraceReporter()
	if err != nil {
		panic(err) // err is always nil as per sources
	}

	return rcmgr.WithTraceReporter(str)
}

type resourceManagerParams struct {
	fx.In

	Limits rcmgr.ConcreteLimitConfig
	Opts   []rcmgr.Option `group:"rcmgr-opts"`
}

func resourceManagerOpt(opt any) fx.Annotated {
	return fx.Annotated{
		Group:  "rcmgr-opts",
		Target: opt,
	}
}
