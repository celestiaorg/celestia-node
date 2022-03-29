package node

// WithRemoteCore configures Node to start with remote Core.
func WithRemoteCore(protocol string, address string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.Core.Protocol = protocol
		cfg.Core.RemoteAddr = address
		return
	}
}

// WithTrustedHash sets TrustedHash to the Config.
func WithTrustedHash(hash string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.Services.TrustedHash = hash
		return
	}
}

// WithTrustedPeers appends new "trusted peers" to the Config.
func WithTrustedPeers(addr ...string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.Services.TrustedPeers = append(cfg.Services.TrustedPeers, addr...)
		return
	}
}

// WithConfig sets the entire custom config.
func WithConfig(custom *Config) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		*cfg = *custom
		return
	}
}

// WithMutualPeers sets the `MutualPeers` field in the config.
func WithMutualPeers(addrs []string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.P2P.MutualPeers = addrs
		return nil
	}
}
