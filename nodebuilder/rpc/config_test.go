package rpc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig tests that the default gateway config is correct.
func TestDefaultConfig(t *testing.T) {
	expected := Config{
		Address:     defaultBindAddress,
		Port:        defaultPort,
		SkipAuth:    false,
		TLSEnabled:  false,
		TLSCertPath: "",
		TLSKeyPath:  "",
	}

	assert.Equal(t, expected, DefaultConfig())
}

func TestConfigValidate(t *testing.T) {
	// Create temporary files for TLS testing
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	
	// Create dummy cert and key files
	os.WriteFile(certFile, []byte("dummy cert"), 0644)
	os.WriteFile(keyFile, []byte("dummy key"), 0644)

	tests := []struct {
		name string
		cfg  Config
		err  bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Address: "127.0.0.1",
				Port:    "8080",
			},
			err: false,
		},
		{
			name: "invalid address",
			cfg: Config{
				Address: "999.999.999.999",
				Port:    "8080",
			},
			err: true,
		},
		{
			name: "invalid port",
			cfg: Config{
				Address: "127.0.0.1",
				Port:    "invalid",
			},
			err: true,
		},
		{
			name: "valid TLS config",
			cfg: Config{
				Address:     "127.0.0.1",
				Port:        "8080",
				TLSEnabled:  true,
				TLSCertPath: certFile,
				TLSKeyPath:  keyFile,
			},
			err: false,
		},
		{
			name: "TLS enabled but missing cert path",
			cfg: Config{
				Address:     "127.0.0.1",
				Port:        "8080",
				TLSEnabled:  true,
				TLSCertPath: "",
				TLSKeyPath:  keyFile,
			},
			err: true,
		},
		{
			name: "TLS enabled but missing key path",
			cfg: Config{
				Address:     "127.0.0.1",
				Port:        "8080",
				TLSEnabled:  true,
				TLSCertPath: certFile,
				TLSKeyPath:  "",
			},
			err: true,
		},
		{
			name: "TLS enabled but cert file doesn't exist",
			cfg: Config{
				Address:     "127.0.0.1",
				Port:        "8080",
				TLSEnabled:  true,
				TLSCertPath: "/nonexistent/cert.pem",
				TLSKeyPath:  keyFile,
			},
			err: true,
		},
		{
			name: "TLS enabled but key file doesn't exist",
			cfg: Config{
				Address:     "127.0.0.1",
				Port:        "8080",
				TLSEnabled:  true,
				TLSCertPath: certFile,
				TLSKeyPath:  "/nonexistent/key.pem",
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.err {
				t.Errorf("Config.Validate() error = %v, err %v", err, tt.err)
			}
		})
	}
}

func TestConfigRequestURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{
			name: "HTTP without TLS",
			cfg: Config{
				Address:    "127.0.0.1",
				Port:       "8080",
				TLSEnabled: false,
			},
			expected: "http://127.0.0.1:8080",
		},
		{
			name: "HTTPS with TLS",
			cfg: Config{
				Address:    "127.0.0.1",
				Port:       "8080",
				TLSEnabled: true,
			},
			expected: "https://127.0.0.1:8080",
		},
		{
			name: "Custom protocol preserved",
			cfg: Config{
				Address:    "ws://127.0.0.1",
				Port:       "8080",
				TLSEnabled: false,
			},
			expected: "ws://127.0.0.1:8080",
		},
		{
			name: "Custom protocol with TLS preserved",
			cfg: Config{
				Address:    "wss://127.0.0.1",
				Port:       "8080",
				TLSEnabled: true,
			},
			expected: "wss://127.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.RequestURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}
