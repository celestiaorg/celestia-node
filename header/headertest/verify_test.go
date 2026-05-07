package headertest

import (
	"strconv"
	"testing"

	tmrand "github.com/cometbft/cometbft/libs/rand"
	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/header"
)

func TestVerify(t *testing.T) {
	h := NewTestSuite(t, 2, 0).GenExtendedHeaders(3)
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
			err: header.ErrValidatorHashMismatch,
		},
		{
			prepare: func() *header.ExtendedHeader {
				untrusted := *untrustedAdj
				untrusted.LastBlockID.Hash = tmrand.Bytes(32)
				return &untrusted
			},
			err: header.ErrLastHeaderHashMismatch,
		},
		{
			prepare: func() *header.ExtendedHeader {
				untrusted := *untrustedNonAdj
				untrusted.Commit = NewTestSuite(t, 2, 0).Commit(RandRawHeader(t))
				return &untrusted
			},
			err: header.ErrVerifyCommitLightTrustingFailed,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := trusted.Verify(test.prepare())
			if test.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, test.err)
		})
	}
}
