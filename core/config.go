package core

import (
	"os"
	"testing"

	"github.com/celestiaorg/celestia-core/config"
)

type Config = config.Config

func DefaultConfig() *Config {
	return config.DefaultConfig()
}

func TestConfig(t *testing.T) *Config {
	cfg := config.ResetTestRoot(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})
	return cfg
}
