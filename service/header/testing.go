package header

import (
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-core/pkg/da"

	tmrand "github.com/celestiaorg/celestia-core/libs/rand"
	tmproto "github.com/celestiaorg/celestia-core/proto/tendermint/types"
	"github.com/celestiaorg/celestia-core/proto/tendermint/version"
	core "github.com/celestiaorg/celestia-core/types"
)

func RandExtendedHeader(t *testing.T) *ExtendedHeader {
	rh := RandRawHeader(t)
	valSet, vals := core.RandValidatorSet(5, 1)
	voteSet := core.NewVoteSet(rh.ChainID, rh.Height, 0, tmproto.PrecommitType, valSet)
	commit, err := core.MakeCommit(RandBlockID(t), rh.Height, 0, voteSet, vals, time.Now())
	require.NoError(t, err)
	dah := da.MinDataAvailabilityHeader()
	return &ExtendedHeader{
		RawHeader:    *rh,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          &dah,
	}
}

func RandRawHeader(t *testing.T) *RawHeader {
	randHash := tmrand.Bytes(32)
	return &RawHeader{
		Version:            version.Consensus{Block: 11, App: 1},
		ChainID:            "test",
		Height:             mrand.Int63(), //nolint:gosec
		Time:               time.Now(),
		LastBlockID:        RandBlockID(t),
		LastCommitHash:     randHash,
		DataHash:           randHash,
		ValidatorsHash:     randHash,
		NextValidatorsHash: randHash,
		ConsensusHash:      randHash,
		AppHash:            randHash,
		LastResultsHash:    randHash,
		EvidenceHash:       randHash,
		ProposerAddress:    tmrand.Bytes(20),
	}
}

func RandBlockID(t *testing.T) core.BlockID {
	bid := core.BlockID{
		Hash: make([]byte, 32),
		PartSetHeader: core.PartSetHeader{
			Total: 123,
			Hash:  make([]byte, 32),
		},
	}
	mrand.Read(bid.Hash)               //nolint:gosec
	mrand.Read(bid.PartSetHeader.Hash) //nolint:gosec
	return bid
}
