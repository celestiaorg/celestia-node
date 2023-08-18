package core

import (
	"fmt"
	"strconv"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	IP       string
	RPCPort  string
	GRPCPort string
}

// DefaultConfig returns default configuration for managing the
// node's connection to a Celestia-Core endpoint.
func DefaultConfig() Config {
	return Config{
		IP:       "",
		RPCPort:  "",
		GRPCPort: "",
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if !cfg.EndpointConfigured() {
		return nil
	}

	ip, err := utils.ValidateAddr(cfg.IP)
	if err != nil {
		return err
	}
	cfg.IP = ip
	_, err = strconv.Atoi(cfg.RPCPort)
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid rpc port: %s", err.Error())
	}
	_, err = strconv.Atoi(cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid grpc port: %s", err.Error())
	}
	return nil
}

// EndpointConfigured returns whether a core endpoint has been set
// on the config (true if set).
func (cfg *Config) EndpointConfigured() bool {
	return cfg.IP != ""
}
