package config

import "time"

// WithRemoteCoreIP configures Node to connect to the given remote Core IP.
func WithRemoteCoreIP(ip string) Option {
	return func(sets *Settings) {
		sets.Cfg.Core.IP = ip
	}
}

// WithRemoteCorePort configures Node to connect to the given remote Core port.
func WithRemoteCorePort(port string) Option {
	return func(sets *Settings) {
		sets.Cfg.Core.RPCPort = port
	}
}

// WithGRPCPort configures Node to connect to given gRPC port
// for state-related queries.
func WithGRPCPort(port string) Option {
	return func(sets *Settings) {
		sets.Cfg.Core.GRPCPort = port
	}
}

// WithRPCPort configures Node to expose the given port for RPC
// queries.
func WithRPCPort(port string) Option {
	return func(sets *Settings) {
		sets.Cfg.RPC.Port = port
	}
}

// WithRPCAddress configures Node to listen on the given address for RPC
// queries.
func WithRPCAddress(addr string) Option {
	return func(sets *Settings) {
		sets.Cfg.RPC.Address = addr
	}
}

// WithTrustedHash sets TrustedHash to the Config.
func WithTrustedHash(hash string) Option {
	return func(sets *Settings) {
		sets.Cfg.Services.TrustedHash = hash
	}
}

// WithTrustedPeers appends new "trusted peers" to the Config.
func WithTrustedPeers(addr ...string) Option {
	return func(sets *Settings) {
		sets.Cfg.Services.TrustedPeers = append(sets.Cfg.Services.TrustedPeers, addr...)
	}
}

// WithPeersLimit overrides default peer limit for peers found during discovery.
func WithPeersLimit(limit uint) Option {
	return func(sets *Settings) {
		sets.Cfg.Services.PeersLimit = limit
	}
}

// WithDiscoveryInterval sets interval between discovery sessions.
func WithDiscoveryInterval(interval time.Duration) Option {
	return func(sets *Settings) {
		if interval <= 0 {
			return
		}
		sets.Cfg.Services.DiscoveryInterval = interval
	}
}

// WithAdvertiseInterval sets interval between advertises.
func WithAdvertiseInterval(interval time.Duration) Option {
	return func(sets *Settings) {
		if interval <= 0 {
			return
		}
		sets.Cfg.Services.AdvertiseInterval = interval
	}
}

// WithConfig sets the entire custom config.
func WithConfig(custom *Config) Option {
	return func(sets *Settings) {
		sets.Cfg = custom
	}
}

// WithMutualPeers sets the `MutualPeers` field in the config.
func WithMutualPeers(addrs []string) Option {
	return func(sets *Settings) {
		sets.Cfg.P2P.MutualPeers = addrs
	}
}
