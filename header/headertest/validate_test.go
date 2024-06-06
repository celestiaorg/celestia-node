package headertest

import (
	"strconv"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		extendedHeader *header.ExtendedHeader
		wantErr        error
	}{
		{
			extendedHeader: getExtendedHeader(t, 1),
			wantErr:        nil,
		},
	}

	for i, test := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := test.extendedHeader.Validate()
			assert.Equal(t, test.wantErr, got)
		})
	}
}

func getExtendedHeader(t *testing.T, appVersion uint64) *header.ExtendedHeader {
	validatorSet, privValidators := RandValidatorSet(1, 1)
	rawHeader := RandRawHeader(t)
	rawHeader.ValidatorsHash = validatorSet.Hash()

	minHeader := da.MinDataAvailabilityHeader()
	rawHeader.DataHash = minHeader.Hash()

	blockID := RandBlockID(t)
	blockID.Hash = rawHeader.Hash()
	voteSet := types.NewVoteSet(rawHeader.ChainID, rawHeader.Height, 0, tmproto.PrecommitType, validatorSet)
	commit, err := MakeCommit(blockID, rawHeader.Height, 0, voteSet, privValidators, time.Now())
	require.NoError(t, err)

	result, err := header.MakeExtendedHeader(rawHeader, commit, validatorSet, nil)
	require.NoError(t, err)
	return result
}
