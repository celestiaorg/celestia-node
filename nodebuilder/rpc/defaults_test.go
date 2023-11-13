package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerDefaultConstants(t *testing.T) {
	assert.Equal(t, "127.0.0.1", defaultBindAddress)
	assert.Equal(t, "26658", defaultPort)
}
