package headertest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"

	"github.com/celestiaorg/celestia-node/header"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		extendedHeader *header.ExtendedHeader
		wantErr        string
	}{
		{
			extendedHeader: getExtendedHeader(t, 1),
			wantErr:        "",
		},
		{
			extendedHeader: getExtendedHeader(t, 2),
			wantErr:        "",
		},
		{
			extendedHeader: getExtendedHeader(t, 3),
			wantErr: "has version 3, this node supports up to version 2. " +
				"Please upgrade to support new version. Note, 0 is not a valid version",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := tc.extendedHeader.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, got)
				return
			}
			assert.ErrorContains(t, got, tc.wantErr)
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
