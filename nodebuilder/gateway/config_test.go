package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig tests that the default gateway config is correct.
func TestDefaultConfig(t *testing.T) {
	expected := Config{
		Address: defaultBindAddress,
		Port:    defaultPort,
		Enabled: false,
	}

	assert.Equal(t, expected, DefaultConfig())
}
