package headertest

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-node/header"
)

func TestVerify(t *testing.T) {
	h := NewTestSuite(t, 2, 0).GenExtendedHeaders(5)

	trustedNonAdjBack, trustedAdjBack := h[0], h[1]
	trusted := h[2]
	untrustedAdj, untrustedNonAdj := h[3], h[4]

	tests := []struct {
		prepare func() *header.ExtendedHeader
		err     error
	}{
		{
			prepare: func() *header.ExtendedHeader { return trustedNonAdjBack },
			err:     nil,
		},
		{
			prepare: func() *header.ExtendedHeader { return trustedAdjBack },
			err:     nil,
		},
		{
			prepare: func() *header.ExtendedHeader { return untrustedAdj },
			err:     nil,
		},
		{
			prepare: func() *header.ExtendedHeader { return untrustedNonAdj },
			err:     nil,
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
				untrusted.RawHeader.LastBlockID.Hash = tmrand.Bytes(32)
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
			assert.ErrorIs(t, errors.Unwrap(err), test.err)
		})
	}
}
