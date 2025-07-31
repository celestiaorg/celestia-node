package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	stateapi "github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state"
)

func TestDisabledStateModule(t *testing.T) {
	// Create a mock state module - for testing we can use a nil interface since we only test error returns
	var mockState stateapi.Module

	disabled := &disabledStateModule{mockState}
	ctx := context.Background()

	// Create test values using zero values
	var testInt state.Int
	var testAddr state.AccAddress
	var testValAddr state.ValAddress
	var testConfig *state.TxConfig

	// Test that all write operations return the read-only mode error
	_, err := disabled.Transfer(ctx, testAddr, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.SubmitPayForBlob(ctx, nil, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.CancelUnbondingDelegation(ctx, testValAddr, testInt, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.BeginRedelegate(ctx, testValAddr, testValAddr, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.Undelegate(ctx, testValAddr, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.Delegate(ctx, testValAddr, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.GrantFee(ctx, testAddr, testInt, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)

	_, err = disabled.RevokeGrantFee(ctx, testAddr, testConfig)
	assert.ErrorIs(t, err, ErrReadOnlyMode)
}

func TestReadOnlyBlobModule(t *testing.T) {
	// Create a mock blob module - for testing we can use a nil interface since we only test error returns
	var mockBlob blobapi.Module

	readOnly := &readOnlyBlobModule{mockBlob}
	ctx := context.Background()

	// Test that Submit operation returns the read-only mode error
	_, err := readOnly.Submit(ctx, nil, nil)
	assert.ErrorIs(t, err, ErrReadOnlyMode)
}

func TestErrorMessages(t *testing.T) {
	require.Equal(t, "node is running in read-only mode", ErrReadOnlyMode.Error())
}
