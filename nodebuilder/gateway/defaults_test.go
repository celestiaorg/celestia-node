package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerDefaultConstants(t *testing.T) {
	assert.Equal(t, "localhost", defaultBindAddress)
	assert.Equal(t, "26659", defaultPort)
}
