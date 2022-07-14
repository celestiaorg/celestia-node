package node

// WithRemoteCore configures Node to start with remote Core.
func WithRemoteCore(ip, port string) Option {
	return func(sets *settings) {
		sets.cfg.Core.IP = ip
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
