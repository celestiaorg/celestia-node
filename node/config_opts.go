package node

import "time"

// WithRemoteCoreIP configures Node to connect to the given remote Core IP.
func WithRemoteCoreIP(ip string) Option {
	return func(sets *settings) {
		sets.cfg.Core.IP = ip
	}
}

// WithRemoteCorePort configures Node to connect to the given remote Core port.
func WithRemoteCorePort(port string) Option {
	return func(sets *settings) {
		sets.cfg.Core.RPCPort = port
	}
}

// WithGRPCPort configures Node to connect to given gRPC port
// for state-related queries.
func WithGRPCPort(port string) Option {
	return func(sets *settings) {
		sets.cfg.Core.GRPCPort = port
	}
}

// WithRPCPort configures Node to expose the given port for RPC
// queries.
func WithRPCPort(port string) Option {
	return func(sets *settings) {
		sets.cfg.RPC.Port = port
	}
}

// WithRPCAddress configures Node to listen on the given address for RPC
// queries.
func WithRPCAddress(addr string) Option {
	return func(sets *settings) {
		sets.cfg.RPC.Address = addr
	}
}

// WithTrustedHash sets TrustedHash to the Config.
func WithTrustedHash(hash string) Option {
	return func(sets *settings) {
		sets.cfg.Services.TrustedHash = hash
	}
}

// WithTrustedPeers appends new "trusted peers" to the Config.
func WithTrustedPeers(addr ...string) Option {
	return func(sets *settings) {
		sets.cfg.Services.TrustedPeers = append(sets.cfg.Services.TrustedPeers, addr...)
	}
}

// WithPeersLimit overrides default peer limit for peers found during discovery.
func WithPeersLimit(limit uint) Option {
	return func(sets *settings) {
		sets.cfg.Services.PeersLimit = limit
	}
}

// WithDiscoveryInterval sets interval between discovery sessions.
func WithDiscoveryInterval(interval time.Duration) Option {
	return func(sets *settings) {
		if interval <= 0 {
			return
		}
		sets.cfg.Services.DiscoveryInterval = interval
	}
}

// WithAdvertiseInterval sets interval between advertises.
func WithAdvertiseInterval(interval time.Duration) Option {
	return func(sets *settings) {
		if interval <= 0 {
			return
		}
		sets.cfg.Services.AdvertiseInterval = interval
	}
}

// WithConfig sets the entire custom config.
func WithConfig(custom *Config) Option {
	return func(sets *settings) {
		sets.cfg = custom
	}
}

// WithMutualPeers sets the `MutualPeers` field in the config.
func WithMutualPeers(addrs []string) Option {
	return func(sets *settings) {
		sets.cfg.P2P.MutualPeers = addrs
	}
}

// WithKeyringAccName sets the `KeyringAccName` field in the key config.
func WithKeyringAccName(name string) Option {
	return func(sets *settings) {
		sets.cfg.Key.KeyringAccName = name
	}
}
