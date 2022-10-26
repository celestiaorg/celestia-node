package gateway

import (
	"fmt"
	"net"
	"strconv"
)

type Config struct {
	Address string
	Port    string
	Enabled bool
}

func DefaultConfig() Config {
	return Config{
		Address: "0.0.0.0",
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port:    "26659",
		Enabled: false,
	}
}

func (cfg *Config) Validate() error {
	if ip := net.ParseIP(cfg.Address); ip == nil {
		return fmt.Errorf("service/rpc: invalid listen address format: %s", cfg.Address)
	}
	_, err := strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("service/rpc: invalid port: %s", err.Error())
	}
	return nil
}
