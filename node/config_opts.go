package node

// WithRemoteCore configures Node to start with remote Core.
func WithRemoteCore(protocol string, address string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.Core.Remote = true
		cfg.Core.RemoteConfig.Protocol = protocol
		cfg.Core.RemoteConfig.RemoteAddr = address
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

// WithTrustedPeer sets TrustedPeer to the Config.
func WithTrustedPeer(addr string) Option {
	return func(cfg *Config, _ *settings) (_ error) {
		cfg.Services.TrustedPeer = addr
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

func WithMutualPeers(addrs []string) Option {
	return func(cfg *Config) (_ error) {
		cfg.P2P.MutualPeers = addrs
		return nil
	}
}
