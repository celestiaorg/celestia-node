package core

import (
	"fmt"
	"strconv"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	DefaultPort = "9090"
)

var MetricsEnabled bool

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	EndpointConfig
	// AdditionalCoreEndpoints lists extra Celestia-Core endpoints beyond the primary one.
	// They serve two purposes: load-balancing transaction submission (see nodebuilder/state),
	// and acting as additional block sources fanned into a MultiSource so the bridge ingests
	// new blocks from every endpoint concurrently and tolerates any single one failing
	// (see core.MultiSource and newCoreFetcher).
	//
	// Configurable only via the config file — there is intentionally no CLI flag. Each entry is
	// a full EndpointConfig (IP, Port, TLSEnabled, XTokenPath): the primary endpoint keeps its
	// --core.* flags for quick start, while this structured, possibly-secured list belongs in
	// config rather than being flattened into flags.
	AdditionalCoreEndpoints []EndpointConfig
}

type EndpointConfig struct {
	IP   string
	Port string
	// TLSEnabled specifies whether the connection is secure.
	// Must be set to true if XTokenPath is provided.
	TLSEnabled bool
	// XTokenPath specifies the path to the directory that contains a JSON file with the X-Token for gRPC authentication.
	// The JSON file can be named either "xtoken.json" or "x-token.json".
	// The JSON file must contain a key "x-token" (preferred) or "xtoken" with the authentication token.
	// If left empty, the client will not include the X-Token in its requests.
	XTokenPath string
}

// DefaultConfig returns default configuration for managing the
// node's connection to a Celestia-Core endpoint.
func DefaultConfig() Config {
	return Config{
		EndpointConfig: EndpointConfig{
			IP:   "",
			Port: DefaultPort,
		},
		AdditionalCoreEndpoints: make([]EndpointConfig, 0),
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if !cfg.IsEndpointConfigured() {
		return nil
	}

	if err := cfg.validate(); err != nil {
		return err
	}

	for _, additionalCfg := range cfg.AdditionalCoreEndpoints {
		if err := additionalCfg.validate(); err != nil {
			return fmt.Errorf("nodebuilder/core: invalid additional core endpoint configuration: %w", err)
		}
	}

	return nil
}

func (cfg *EndpointConfig) validate() error {
	if cfg.Port == "" {
		return fmt.Errorf("nodebuilder/core: grpc port is not set")
	}

	ip, err := utils.SanitizeAddr(cfg.IP)
	if err != nil {
		return err
	}
	cfg.IP = ip
	_, err = strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("nodebuilder/core: invalid grpc port: %s", err.Error())
	}

	if cfg.XTokenPath != "" {
		if !cfg.TLSEnabled {
			return fmt.Errorf("nodebuilder/core: TLSEnabled must be true when XTokenPath is set")
		}
		if !utils.Exists(cfg.XTokenPath) {
			return fmt.Errorf("nodebuilder/core: XTokenPath does not exist: %s", cfg.XTokenPath)
		}
	}

	return nil
}

// IsEndpointConfigured returns whether a core endpoint has been set
// on the config (true if set).
func (cfg *Config) IsEndpointConfigured() bool {
	return cfg.IP != ""
}
