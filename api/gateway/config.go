package gateway

import (
	"fmt"
	"net"
	"strconv"
)

type Config struct {
	Address string
	Port    string
}

func (cfg *Config) Validate() error {
	if ip := net.ParseIP(cfg.Address); ip == nil {
		return fmt.Errorf("service/gateway: invalid listen address format: %s", cfg.Address)
	}
	_, err := strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("service/gateway: invalid port: %s", err.Error())
	}
	return nil
}
