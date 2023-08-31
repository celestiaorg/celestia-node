package headertest

import (
	"strconv"
	"testing"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/go-header"
	"github.com/stretchr/testify/assert"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

func TestVerify(t *testing.T) {
	h := NewTestSuite(t, 2).GenExtendedHeaders(3)
	trusted, untrustedAdj, untrustedNonAdj := h[0], h[1], h[2]
	tests := []struct {
		prepare func() *header.ExtendedHeader
		err     error
	}{
		{
			prepare: func() *header.ExtendedHeader { return untrustedAdj },
			err:     nil,
		},
		{
			prepare: func() *header.ExtendedHeader {
				return untrustedNonAdj
			},
			err: nil,
		},
		{
			prepare: func() *header.ExtendedHeader {
				untrusted := *untrustedAdj
				untrusted.ValidatorsHash = tmrand.Bytes(32)
				return &untrusted
			},
			err: &libhead.VerifyError{
				Reason: &header.ErrValidatorHashMismatch{},
			},
		},
		{
			prepare: func() *header.ExtendedHeader {
				untrusted := *untrustedAdj
				untrusted.RawHeader.LastBlockID.Hash = tmrand.Bytes(32)
				return &untrusted
			},
			err: &libhead.VerifyError{
				Reason: &header.ErrLastHeaderHashMismatch{},
			},
		},
		{
			prepare: func() *header.ExtendedHeader {
				untrusted := *untrustedNonAdj
				untrusted.Commit = NewTestSuite(t, 2).Commit(RandRawHeader(t))
				return &untrusted
			},
			err: &libhead.VerifyError{
				Reason: &types.ErrNotEnoughVotingPowerSigned{},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := trusted.Verify(test.prepare())
			// Case 1 & 2: test.err == nil and err == nil or err != nil
			if test.err == nil {
				assert.NoError(t, err)
				return
			}
			// Case 3: test.err != nil and err != nil
			if err == nil {
				t.Errorf("expected err: %v, got nil", test.err)
				return
			}
			// Case 4: test.err != nil and err != nil
			switch (err).(type) {
			case *libhead.VerifyError:
				reason := err.(*libhead.VerifyError).Reason
				testReason := test.err.(*libhead.VerifyError).Reason
				switch reason.(type) {
				case *header.ErrValidatorHashMismatch:
					assert.Equal(t, &header.ErrValidatorHashMismatch{}, testReason)
				case *header.ErrLastHeaderHashMismatch:
					assert.Equal(t, &header.ErrLastHeaderHashMismatch{}, testReason)
				case types.ErrNotEnoughVotingPowerSigned:
					assert.Equal(t, &types.ErrNotEnoughVotingPowerSigned{}, testReason)
				default:
					assert.Equal(t, testReason, reason)
				}
			default:
				assert.Failf(t, "unexpected error: %s\n", err.Error())
			}
		})
	}
}
