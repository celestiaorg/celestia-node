package rpc

import (
	"fmt"
	"strconv"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type Config struct {
	Address string
	Port    string
}

func DefaultConfig() Config {
	return Config{
		Address: "0.0.0.0",
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port: "26658",
	}
}

func (cfg *Config) Validate() error {
	sanitizedAddress, err := utils.ValidateAddr(cfg.Address)
	if err != nil {
		return fmt.Errorf("service/rpc: invalid address: %w", err)
	}
	cfg.Address = sanitizedAddress

	_, err = strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("service/rpc: invalid port: %s", err.Error())
	}
	return nil
}
