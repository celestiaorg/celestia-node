package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"go.uber.org/fx"
)

func resourceManager(params resourceManagerParams) (network.ResourceManager, error) {
	return rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(params.Limits))
}

func infiniteResources() rcmgr.ConcreteLimitConfig {
	return rcmgr.InfiniteLimits
}

func autoscaleResources() rcmgr.ConcreteLimitConfig {
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	return limits.AutoScale()
}

func allowList(ctx context.Context, cfg Config, bootstrappers Bootstrappers) (rcmgr.Option, error) {
	mutual, err := cfg.mutualPeers()
	if err != nil {
		return nil, err
	}

	// TODO(@Wondertan): We should resolve their addresses only once, but currently
	//  we resolve it here and libp2p stuck does that as well internally
	allowlist := make([]ma.Multiaddr, 0, len(bootstrappers)+len(mutual))
	for _, b := range bootstrappers {
		for _, baddr := range b.Addrs {
			resolved, err := madns.DefaultResolver.Resolve(ctx, baddr)
			if err != nil {
				log.Warnw("error resolving bootstrapper DNS", "addr", baddr.String(), "err", err)
				continue
			}
			allowlist = append(allowlist, resolved...)
		}
	}
	for _, m := range mutual {
		for _, maddr := range m.Addrs {
			resolved, err := madns.DefaultResolver.Resolve(ctx, maddr)
			if err != nil {
				log.Warnw("error resolving mutual peer DNS", "addr", maddr.String(), "err", err)
				continue
			}
			allowlist = append(allowlist, resolved...)
		}
	}

	return rcmgr.WithAllowlistedMultiaddrs(allowlist), nil
}

func traceReporter() rcmgr.Option {
	str, err := rcmgrObs.NewStatsTraceReporter()
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
