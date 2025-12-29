package blob

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOnlyBlobModule(t *testing.T) {
	// Create a mock blob module - for testing we can use a nil interface since we only test error returns
	var mockBlob Module

	readOnly := &readOnlyBlobModule{mockBlob}
	ctx := context.Background()

	// Test that Submit operation returns the read-only mode error
	_, err := readOnly.Submit(ctx, nil, nil)
	assert.ErrorIs(t, err, ErrReadOnlyMode)
}

func TestBlobErrorMessages(t *testing.T) {
	require.Equal(t, "node is running in read-only mode", ErrReadOnlyMode.Error())
}
