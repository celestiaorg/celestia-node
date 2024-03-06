package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var (
	MetricsEnabled bool

	ErrMultipleHostsConfigured = errors.New("multiple hosts configured")
)

const (
	DefaultRPCScheme = "http"
	DefaultRPCPort   = "26657"
	DefaultGRPCPort  = "9090"
)

// Config combines all configuration fields for
// managing the relationship with a Core node.
type Config struct {
	IP           string
	RPC          HostConfig
	GRPC         HostConfig
	OtherRPCHost string
}

type HostConfig struct {
	Scheme string
	Host   string
	Port   string
	Cert   string
}

// DefaultConfig returns default configuration for managing the
// node's connection to a Celestia-Core endpoint.
func DefaultConfig() Config {
	return Config{
		IP: "",
		RPC: HostConfig{
			Scheme: DefaultRPCScheme,
			Port:   DefaultRPCPort,
		},
		GRPC: HostConfig{
			Scheme: DefaultRPCScheme,
			Port:   DefaultGRPCPort,
		},
	}
}

func (cfg *Config) RPCHost() string {
	if cfg.RPC.Host != "" {
		return cfg.RPC.Host
	}
	return cfg.IP
}

func (cfg *Config) GRPCHost() string {
	if cfg.GRPC.Host != "" {
		return cfg.GRPC.Host
	}
	return cfg.IP
}

func (cfg *Config) multipleHostsConfigured() error {
	if cfg.IP != "" && cfg.RPC.Host != "" {
		return fmt.Errorf(
			"%w: core.ip overridden by core.rpc.host",
			ErrMultipleHostsConfigured,
		)
	}

	if cfg.IP != "" && cfg.GRPC.Host != "" {
		return fmt.Errorf(
			"%w: core.ip overridden by core.grpc.host",
			ErrMultipleHostsConfigured,
		)
	}

	return nil
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if !cfg.IsEndpointConfigured() {
		return nil
	}

	if err := cfg.multipleHostsConfigured(); err != nil {
		return err
	}

	_, err := utils.ValidateAddr(cfg.RPCHost())
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid rpc host: %s", err.Error())
	}

	rpcURL, err := url.Parse(cfg.RPCHost())
	if rpcURL.Scheme != "" {
		cfg.RPC.Scheme = rpcURL.Scheme
	}

	grpcHost, _ := utils.ValidateAddr(cfg.GRPCHost())
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid grpc host: %s", err.Error())
	}

	cfg.GRPC.Host = grpcHost

	grpcURL, _ := url.Parse(cfg.GRPCHost())
	if grpcURL.Scheme != "" {
		cfg.GRPC.Scheme = rpcURL.Scheme
	}

	_, err = strconv.Atoi(cfg.RPC.Port)
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid rpc port: %s", err.Error())
	}
	_, err = strconv.Atoi(cfg.GRPC.Port)
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid grpc port: %s", err.Error())
	}

	if cfg.GRPC.Cert != "" {
		if _, err := os.Stat(cfg.GRPC.Cert); os.IsNotExist(err) {
			return fmt.Errorf("nodebuilder/core: grpc cert file does not exist: %s", cfg.GRPC.Cert)
		}
	}

	return nil
}

// IsEndpointConfigured returns whether a core endpoint has been set
// on the config (true if set).
func (cfg *Config) IsEndpointConfigured() bool {
	fmt.Println(cfg)
	fmt.Println(cfg.RPCHost())
	fmt.Println(cfg.GRPCHost())
	return cfg.RPCHost() != "" && cfg.GRPCHost() != ""
}
