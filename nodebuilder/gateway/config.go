package gateway

import (
	"fmt"
	"strconv"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type Config struct {
	Address string
	Port    string
	Enabled bool
}

func DefaultConfig() Config {
	return Config{
		Address: defaultBindAddress,
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port:    defaultPort,
		Enabled: false,
	}
}

func (cfg *Config) Validate() error {
	sanitizedAddress, err := utils.ValidateAddr(cfg.Address)
	if err != nil {
		return fmt.Errorf("gateway: invalid address: %w", err)
	}
	cfg.Address = sanitizedAddress

	_, err = strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("gateway: invalid port: %s", err.Error())
	}
	return nil
}
