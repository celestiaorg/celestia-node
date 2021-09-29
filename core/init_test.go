package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO(@Bidon15): We need more test coverage for Init and IsInit part
// Tests could include valid/invalid paths and mocks for files
// For more info visit #90
func TestIsInit(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir)
	require.NoError(t, err)
	ok := IsInit(dir)
	assert.True(t, ok)
}
