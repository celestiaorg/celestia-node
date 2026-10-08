package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeCommandUsage(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		use  string
	}{
		{name: "bridge", cmd: NewBridge().Use, use: "bridge [command]"},
		{name: "light", cmd: NewLight().Use, use: "light [command]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.use, tt.cmd)
		})
	}
}

func TestAuthCommandUsage(t *testing.T) {
	assert.Equal(t, "auth (public | read | write | admin)", AuthCmd().Use)
}
