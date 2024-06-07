package headertest

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/header"
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
		{
			extendedHeader: getExtendedHeader(t, 2),
			wantErr:        nil,
		},
		{
			extendedHeader: getExtendedHeader(t, 3),
			wantErr:        fmt.Errorf("app version mismatch, expected: 1 or 2, got 3"),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := tc.extendedHeader.Validate()
			assert.Equal(t, tc.wantErr, got)
		})
	}
}

func getExtendedHeader(t *testing.T, appVersion uint64) *header.ExtendedHeader {
	validatorSet, privValidators := RandValidatorSet(1, 1)
	rawHeader := RandRawHeader(t)
	rawHeader.Version.App = appVersion
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
