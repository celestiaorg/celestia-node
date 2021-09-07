package core

import "github.com/celestiaorg/celestia-core/config"

type Config = config.Config

func DefaultConfig() *Config {
	return config.DefaultConfig()
}

func TestConfig(tname string) *Config {
	return config.ResetTestRoot(tname)
}
