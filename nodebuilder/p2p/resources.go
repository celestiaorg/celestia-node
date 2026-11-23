package p2p

import (
	"context"
	"slices"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	manet "github.com/multiformats/go-multiaddr/net"
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

func allowList(ctx context.Context, cfg *Config, bootstrappers Bootstrappers) (rcmgr.Option, error) {
	mutual, err := cfg.mutualPeers()
	if err != nil {
		return nil, err
	}

	// TODO(@Wondertan): We should resolve their addresses only once, but currently
	//  we resolve it here and libp2p stuck does that as well internally
	allowlist := make([]ma.Multiaddr, 0, len(bootstrappers)+len(mutual))
	for _, b := range bootstrappers {
		p2p := ma.StringCast("/p2p/" + b.ID.String())
		for _, baddr := range b.Addrs {
			resolved, err := resolveIPs(ctx, baddr.Encapsulate(p2p))
			if err != nil {
				log.Warnw("error resolving bootstrapper DNS", "addr", baddr.String(), "err", err)
				continue
			}
			allowlist = append(allowlist, resolved...)
		}
	}
	for _, m := range mutual {
		p2p := ma.StringCast("/p2p/" + m.ID.String())
		for _, maddr := range m.Addrs {
			resolved, err := resolveIPs(ctx, maddr.Encapsulate(p2p))
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

// resolveIPs resolves addr until no DNS components remain and drops addresses without an IP.
// A /dnsaddr record resolves to /dns addresses, which need one more resolution step.
func resolveIPs(ctx context.Context, addr ma.Multiaddr) ([]ma.Multiaddr, error) {
	addrs := []ma.Multiaddr{addr}
	for range 3 {
		var next []ma.Multiaddr
		for _, a := range addrs {
			if !madns.Matches(a) {
				next = append(next, a)
				continue
			}
			resolved, err := madns.DefaultResolver.Resolve(ctx, a)
			if err != nil {
				return nil, err
			}
			next = append(next, resolved...)
		}
		addrs = next
	}
	return slices.DeleteFunc(addrs, func(a ma.Multiaddr) bool {
		_, err := manet.ToIP(a)
		return err != nil
	}), nil
}
