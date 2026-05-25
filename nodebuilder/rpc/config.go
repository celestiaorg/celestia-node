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

// RateLimitConfig configures per-IP rate limiting on the RPC server. The
// limit is keyed by the connection's remote address, so behind a reverse
// proxy all clients share a single bucket — keep this disabled in that
// setup and apply rate limiting at the proxy instead.
type RateLimitConfig struct {
	Enabled bool
	// RequestsPerSec is the sustained per-IP request rate; requests above
	// this rate (after burst is drained) get 429 Too Many Requests.
	RequestsPerSec int
	// Burst is the per-IP burst allowance — the bucket size that absorbs
	// short spikes before sustained rate kicks in.
	Burst int
	// CacheSize is the max number of per-IP buckets retained.
	// When exceeded, the least-recently-seen IP is evicted (and gets a fresh
	// burst on its next request). Bounds memory under unique-IP floods.
	CacheSize int
}

type Config struct {
	Address     string
	Port        string
	SkipAuth    bool
	CORS        CORSConfig
	TLSEnabled  bool
	TLSCertPath string
	TLSKeyPath  string
	RateLimit   RateLimitConfig
}

func DefaultConfig() Config {
	return Config{
		Address: defaultBindAddress,
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port:      defaultPort,
		SkipAuth:  false,
		CORS:      DefaultCORSConfig(),
		RateLimit: DefaultRateLimitConfig(),
	}
}

// DefaultRateLimitConfig disables rate limiting by default; production
// deployments typically run behind a reverse proxy that handles rate
// limiting correctly (with the real client IP).
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:        false,
		RequestsPerSec: 100,
		Burst:          200,
		CacheSize:      8192,
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

	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.RequestsPerSec <= 0 || cfg.RateLimit.Burst <= 0 {
			return fmt.Errorf("service/rpc: rate limit RequestsPerSec and Burst must be > 0")
		}
		if cfg.RateLimit.CacheSize <= 0 {
			return fmt.Errorf("service/rpc: rate limit CacheSize must be > 0")
		}
	}

	return nil
}
