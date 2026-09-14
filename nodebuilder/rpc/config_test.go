package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const ipv4Loopback = "127.0.0.1"

// TestDefaultConfig tests that the default rpc config is correct.
func TestDefaultConfig(t *testing.T) {
	expected := Config{
		Address:  defaultBindAddress,
		Port:     defaultPort,
		SkipAuth: false,
		CORS: CORSConfig{
			Enabled:        false,
			AllowedOrigins: []string{},
			AllowedHeaders: []string{},
			AllowedMethods: []string{},
		},
		RateLimit: DefaultRateLimitConfig(),
	}

	assert.Equal(t, expected, DefaultConfig())
}

func TestRequestURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "IPv4",
			cfg:  Config{Address: ipv4Loopback, Port: "8080"},
			want: "http://127.0.0.1:8080",
		},
		{
			name: "IPv6",
			cfg:  Config{Address: "::1", Port: "8080"},
			want: "http://[::1]:8080",
		},
		{
			name: "bracketed IPv6",
			cfg:  Config{Address: "[::1]", Port: "8080"},
			want: "http://[::1]:8080",
		},
		{
			name: "TLS IPv6",
			cfg:  Config{Address: "2001:db8::1", Port: "8080", TLSEnabled: true},
			want: "https://[2001:db8::1]:8080",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.cfg.RequestURL())
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		err  bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Address: ipv4Loopback,
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
				Address: ipv4Loopback,
				Port:    "invalid",
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
