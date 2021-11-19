package node

// Option for Node's Config.
type Option func(*Config)

// WithRemoteCore configures Node to start with remote Core.
func WithRemoteCore(protocol string, address string) Option {
	return func(cfg *Config) {
		cfg.Core.Remote = true
		cfg.Core.RemoteConfig.Protocol = protocol
		cfg.Core.RemoteConfig.RemoteAddr = address
	}
}

// WithGenesis sets GenesisHash to the Config.
func WithGenesis(hash string) Option {
	return func(cfg *Config) {
		cfg.Services.GenesisHash = hash
	}
}

// WithTrustedPeer sets TrustedPeer to the Config.
func WithTrustedPeer(addr string) Option {
	return func(cfg *Config) {
		cfg.Services.TrustedPeer = addr
	}
}

// WithConfig sets the entire custom config.
func WithConfig(custom *Config) Option {
	return func(cfg *Config) {
		*cfg = *custom
	}
}
