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

// Endpoint represents a single core gRPC endpoint
type Endpoint struct {
	// IP address of the endpoint
	IP string
	// Port number for the gRPC service
	Port string
	// TLSEnabled specifies whether to use TLS for the connection
	TLSEnabled bool
	// XTokenPath specifies the path to the authentication token file
	XTokenPath string
}

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	// Deprecated: Legacy field for backward compatibility. Use Endpoints instead.
	// IP address for a single core endpoint.
	IP string
	// Deprecated: Legacy field for backward compatibility. Use Endpoints instead.
	// Port number for a single core endpoint.
	Port string
	// Deprecated: Legacy field for backward compatibility. Use Endpoints instead.
	// TLSEnabled specifies whether the connection is secure or not.
	// PLEASE NOTE: it should be set to true in order to handle XTokenPath.
	TLSEnabled bool
	// Deprecated: Legacy field for backward compatibility. Use Endpoints instead.
	// XTokenPath specifies the path to the directory with JSON file containing the X-Token for gRPC authentication.
	// The JSON file should have a key-value pair where the key is "x-token" and the value is the authentication token.
	// If left empty, the client will not include the X-Token in its requests.
	XTokenPath string

	// Endpoints is a list of core endpoints to connect to.
	// When multiple endpoints are configured, connections are load-balanced using round-robin.
	Endpoints []Endpoint
}

// DefaultConfig returns default configuration for managing the
// node's connection to a Celestia-Core endpoint.
func DefaultConfig() Config {
	return Config{
		IP:        "",
		Port:      DefaultPort,
		Endpoints: []Endpoint{}, // Empty by default
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	// If the primary endpoint is configured, validate it
	if cfg.IsLegacyEndpointConfigured() {
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
			return fmt.Errorf("nodebuilder/core: invalid grpc port: %w", err)
		}
	}

	// Validate each endpoint in the new format
	for i, endpoint := range cfg.Endpoints {
		if endpoint.IP == "" {
			return fmt.Errorf("nodebuilder/core: endpoint at index %d has empty IP", i)
		}
		if endpoint.Port == "" {
			return fmt.Errorf("nodebuilder/core: grpc port is not set for endpoint at index %d", i)
		}

		ip, err := utils.SanitizeAddr(endpoint.IP)
		if err != nil {
			return fmt.Errorf("nodebuilder/core: invalid IP for endpoint at index %d: %w", i, err)
		}

		// Update the sanitized IP
		cfg.Endpoints[i].IP = ip

		_, err = strconv.Atoi(endpoint.Port)
		if err != nil {
			return fmt.Errorf("nodebuilder/core: invalid grpc port for endpoint at index %d: %w", i, err)
		}
	}

	return nil
}

// IsLegacyEndpointConfigured returns whether a legacy core endpoint has been set
// on the config (true if set).
func (cfg *Config) IsLegacyEndpointConfigured() bool {
	return cfg.IP != ""
}

// GetAllEndpoints returns all configured endpoints (both legacy and new)
// It combines the legacy endpoint (if configured) with all endpoints from the
// Endpoints slice.
func (cfg *Config) GetAllEndpoints() []Endpoint {
	endpoints := make([]Endpoint, 0)

	// Add legacy endpoint if configured
	if cfg.IsLegacyEndpointConfigured() {
		endpoints = append(endpoints, Endpoint{
			IP:         cfg.IP,
			Port:       cfg.Port,
			TLSEnabled: cfg.TLSEnabled,
			XTokenPath: cfg.XTokenPath,
		})
	}

	// Add all endpoints from the new field
	endpoints = append(endpoints, cfg.Endpoints...)

	return endpoints
}
