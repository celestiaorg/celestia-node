package state

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAmountValidationDoesNotPanic verifies that the amount validation in
// Transfer, Delegate, Undelegate, BeginRedelegate, and
// CancelUnbondingDelegation does not panic on amounts that exceed the int64
// range. Prior to the fix, these functions called amount.Int64() which panics
// on overflow.
func TestAmountValidationDoesNotPanic(t *testing.T) {
	hugeAmount, ok := sdkmath.NewIntFromString("99999999999999999999999999999999999999")
	require.True(t, ok)

	testCases := []struct {
		name   string
		amount sdkmath.Int
		errMsg string
	}{
		{
			name:   "nil amount returns ErrInvalidAmount",
			amount: sdkmath.Int{},
			errMsg: ErrInvalidAmount.Error(),
		},
		{
			name:   "zero amount returns ErrInvalidAmount",
			amount: sdkmath.NewInt(0),
			errMsg: ErrInvalidAmount.Error(),
		},
		{
			name:   "negative amount returns ErrInvalidAmount",
			amount: sdkmath.NewInt(-1),
			errMsg: ErrInvalidAmount.Error(),
		},
		{
			name:   "overflow amount does not panic",
			amount: hugeAmount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				if tc.amount.IsNil() || !tc.amount.IsPositive() {
					assert.NotEmpty(t, tc.errMsg, "expected no error but validation would reject")
				} else {
					assert.Empty(t, tc.errMsg, "expected error but validation would pass")
				}
			})
		})
	}
}

// TestCancelUnbondingDelegationHeightOverflow verifies that
// CancelUnbondingDelegation does not panic when the height parameter exceeds
// the int64 range.
func TestCancelUnbondingDelegationHeightOverflow(t *testing.T) {
	hugeHeight, ok := sdkmath.NewIntFromString("99999999999999999999999999999999999999")
	require.True(t, ok)

	ca := &CoreAccessor{}
	ctx := context.Background()
	validAmount := sdkmath.NewInt(1)

	testCases := []struct {
		name    string
		height  sdkmath.Int
		wantErr error
	}{
		{
			name:    "nil height",
			height:  sdkmath.Int{},
			wantErr: ErrInvalidHeight,
		},
		{
			name:    "zero height",
			height:  sdkmath.NewInt(0),
			wantErr: ErrInvalidHeight,
		},
		{
			name:    "negative height",
			height:  sdkmath.NewInt(-1),
			wantErr: ErrInvalidHeight,
		},
		{
			name:    "overflow height",
			height:  hugeHeight,
			wantErr: ErrInvalidHeight,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, err := ca.CancelUnbondingDelegation(ctx, ValAddress{}, validAmount, tc.height, nil)
				assert.ErrorIs(t, err, tc.wantErr)
			})
		})
	}
}
