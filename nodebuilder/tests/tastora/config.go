package tastora

// Config represents configuration options for the Tastora testing framework.
type Config struct {
	NumValidators    int
	BridgeNodeCount  int
	LightNodeCount   int
	TxWorkerAccounts int // Number of parallel transaction submission lanes
}

// Option for modifying Tastora's Config.
type Option func(*Config)

// defaultConfig creates the default configuration for Tastora tests.
func defaultConfig() *Config {
	return &Config{
		NumValidators:    1,
		BridgeNodeCount:  1,
		LightNodeCount:   1,
		TxWorkerAccounts: 0, // Default: direct submission (no queuing)
	}
}

// WithValidators sets the number of validators for the chain.
func WithValidators(count int) Option {
	return func(c *Config) {
		c.NumValidators = count
	}
}

// WithBridgeNodes sets the number of bridge nodes in the DA network.
func WithBridgeNodes(count int) Option {
	return func(c *Config) {
		c.BridgeNodeCount = count
	}
}

// WithLightNodes sets the number of light nodes in the DA network.
func WithLightNodes(count int) Option {
	return func(c *Config) {
		c.LightNodeCount = count
	}
}

// WithTxWorkerAccounts sets the number of parallel transaction submission lanes.
// 0 = direct submission (no queuing), 1 = queued submission, >1 = parallel submission
func WithTxWorkerAccounts(count int) Option {
	return func(c *Config) {
		c.TxWorkerAccounts = count
	}
}
