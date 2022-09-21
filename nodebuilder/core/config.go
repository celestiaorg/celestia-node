package core

import (
	"fmt"
	"strconv"
	"strings"
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
		IP:       "0.0.0.0",
		RPCPort:  "0",
		GRPCPort: "0",
	}
}

// ValidateBasic performs basic validation of the config.
func (cfg *Config) ValidateBasic() error {
	ip, err := sanitizeIP(cfg.IP)
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

// sanitizeIP trims leading protocol scheme and port from the given
// IP address if present.
func sanitizeIP(ip string) (string, error) {
	original := ip
	ip = strings.TrimPrefix(ip, "http://")
	ip = strings.TrimPrefix(ip, "https://")
	ip = strings.TrimPrefix(ip, "tcp://")
	ip = strings.TrimSuffix(ip, "/")
	ip = strings.Split(ip, ":")[0]
	if ip == "" {
		return "", fmt.Errorf("nodebuilder/core: invalid IP addr given: %s", original)
	}
	return ip, nil
}
