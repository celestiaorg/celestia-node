package rpc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type Config struct {
	Address     string
	Port        string
	SkipAuth    bool
	TLSEnabled  bool
	TLSCertPath string
	TLSKeyPath  string
}

func DefaultConfig() Config {
	return Config{
		Address: defaultBindAddress,
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port:        defaultPort,
		SkipAuth:    false,
		TLSEnabled:  false,
		TLSCertPath: "",
		TLSKeyPath:  "",
	}
}

func (cfg *Config) RequestURL() string {
	protocol := "http"
	if cfg.TLSEnabled {
		protocol = "https"
	}
	if strings.HasPrefix(cfg.Address, "http://") || strings.HasPrefix(cfg.Address, "https://") {
		parts := strings.Split(cfg.Address, "://")
		return fmt.Sprintf("%s://%s:%s", parts[0], parts[1], cfg.Port)
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
			return fmt.Errorf("service/rpc: TLS enabled but cert or key path not set")
		}
	}
	return nil
}
