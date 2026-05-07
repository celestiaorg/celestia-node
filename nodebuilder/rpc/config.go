package rpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type CORSConfig struct {
	Enabled        bool
	AllowedOrigins []string
	AllowedHeaders []string
	AllowedMethods []string
}

type Config struct {
	Address     string
	Port        string
	SkipAuth    bool
	CORS        CORSConfig
	TLSEnabled  bool
	TLSCertPath string
	TLSKeyPath  string
}

func DefaultConfig() Config {
	return Config{
		Address: defaultBindAddress,
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port:     defaultPort,
		SkipAuth: false,
		CORS:     DefaultCORSConfig(),
	}
}

func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		Enabled:        false,
		AllowedOrigins: []string{},
		AllowedHeaders: []string{},
		AllowedMethods: []string{},
	}
}

func (cfg *Config) RequestURL() string {
	if strings.HasPrefix(cfg.Address, "://") {
		parts := strings.Split(cfg.Address, "://")
		return fmt.Sprintf("%s://%s:%s", parts[0], parts[1], cfg.Port)
	}

	protocol := "http"
	if cfg.TLSEnabled {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%s", protocol, cfg.Address, cfg.Port)
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

	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
			return fmt.Errorf("service/rpc: TLS certificate and key paths must be specified when TLS is enabled")
		}
		if _, err := os.Stat(cfg.TLSCertPath); err != nil {
			return fmt.Errorf("service/rpc: TLS certificate file error: %w", err)
		}
		if _, err := os.Stat(cfg.TLSKeyPath); err != nil {
			return fmt.Errorf("service/rpc: TLS key file error: %w", err)
		}
	}

	return nil
}
