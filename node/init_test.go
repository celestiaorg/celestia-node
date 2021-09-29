package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO(@Bidon15): We need more test coverage for Init part
// Tests could include invalid paths/configs and custom ones
// For more info visit #89 
func TestInit(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir, DefaultFullConfig(), Full)
	require.NoError(t, err)
	ok := IsInit(dir)
	assert.True(t, ok)
}
