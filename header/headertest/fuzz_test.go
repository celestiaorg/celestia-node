package headertest

import (
	"bytes"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/cometbft/cometbft/types"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/da"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

// fuzzChain is a deterministic chain: fixed validator keys, timestamps and
// hashes, so every fuzz worker and every replay of a saved input sees the same
// trusted header.
type fuzzChain struct {
	t       testing.TB
	vals    []types.PrivValidator
	valSet  *types.ValidatorSet
	genesis time.Time
}

func newFuzzChain(tb testing.TB) *fuzzChain {
	tb.Helper()
	const numVals = 4
	valz := make([]*types.Validator, numVals)
	privs := make([]types.PrivValidator, numVals)
	for i := range numVals {
		priv := types.NewMockPVWithParams(ed25519.GenPrivKeyFromSecret([]byte{'v', byte(i)}), false, false)
		pub, err := priv.GetPubKey()
		require.NoError(tb, err)
		valz[i] = types.NewValidator(pub, defaultVotingPower)
		privs[i] = priv
	}
	// MakeCommit indexes validators by position, so order them as the set does.
	valSet := types.NewValidatorSet(valz)
	ordered := make([]types.PrivValidator, numVals)
	for i, priv := range privs {
		pub, err := priv.GetPubKey()
		require.NoError(tb, err)
		idx, _ := valSet.GetByAddress(pub.Address())
		require.GreaterOrEqual(tb, idx, int32(0))
		ordered[idx] = privs[i]
	}
	return &fuzzChain{
		t:       tb,
		vals:    ordered,
		valSet:  valSet,
		genesis: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (c *fuzzChain) header(
	height int64,
	prev *header.ExtendedHeader,
	dah *da.DataAvailabilityHeader,
) *header.ExtendedHeader {
	c.t.Helper()
	fill := func(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }
	raw := &header.RawHeader{
		Version:            version.Consensus{Block: 11, App: 1},
		ChainID:            "fuzz",
		Height:             height,
		Time:               c.genesis.Add(time.Duration(height) * time.Second),
		LastBlockID:        types.BlockID{Hash: fill(1), PartSetHeader: types.PartSetHeader{Total: 1, Hash: fill(2)}},
		LastCommitHash:     fill(3),
		DataHash:           dah.Hash(),
		ValidatorsHash:     c.valSet.Hash(),
		NextValidatorsHash: c.valSet.Hash(),
		ConsensusHash:      fill(4),
		AppHash:            fill(5),
		LastResultsHash:    fill(6),
		EvidenceHash:       tmhash.Sum([]byte{}),
		ProposerAddress:    c.valSet.Validators[0].Address,
	}
	if prev != nil {
		raw.LastBlockID = prev.Commit.BlockID
		raw.LastCommitHash = prev.Commit.Hash()
	}
	blockID := types.BlockID{Hash: raw.Hash(), PartSetHeader: types.PartSetHeader{Total: 1, Hash: fill(7)}}
	voteSet := types.NewVoteSet(raw.ChainID, raw.Height, 0, tmproto.PrecommitType, c.valSet)
	commit, err := MakeCommit(blockID, raw.Height, 0, voteSet, c.vals, raw.Time)
	require.NoError(c.t, err)
	eh := &header.ExtendedHeader{RawHeader: *raw, Commit: commit, ValidatorSet: c.valSet, DAH: dah}
	require.NoError(c.t, eh.Validate())
	return eh
}

// FuzzExtendedHeader decodes an ExtendedHeader the way the header exchange and
// header pubsub do, computes its pubsub message ID, validates it, and verifies
// it against a trusted header. A decoded header must re-encode to the same
// bytes it decodes from, and nothing may panic.
func FuzzExtendedHeader(f *testing.F) {
	chain := newFuzzChain(f)
	empty := share.EmptyEDSRoots()
	trusted := chain.header(1, nil, empty)
	adjacent := chain.header(2, trusted, empty)
	nonAdjacent := chain.header(5, nil, empty)
	minimal := da.MinDataAvailabilityHeader()
	withMinDAH := chain.header(3, nil, &minimal)

	for _, eh := range []*header.ExtendedHeader{trusted, adjacent, nonAdjacent, withMinDAH} {
		raw, err := eh.MarshalBinary()
		require.NoError(f, err)
		f.Add(raw)
		f.Add(raw[:len(raw)/2])
	}
	// A header whose DataHash does not match its DAH.
	tampered := *adjacent
	tampered.DataHash = bytes.Repeat([]byte{9}, 32)
	raw, err := tampered.MarshalBinary()
	require.NoError(f, err)
	f.Add(raw)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		// Every gossiped message gets an ID before it is validated.
		_ = header.MsgID(&pb.Message{Data: raw})

		var eh header.ExtendedHeader
		if err := eh.UnmarshalBinary(raw); err != nil {
			return
		}
		if encoded, err := eh.MarshalBinary(); err == nil {
			var again header.ExtendedHeader
			require.NoError(t, again.UnmarshalBinary(encoded), "re-encoded header must decode")
			reencoded, err := again.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, encoded, reencoded, "header encoding is not stable")
		}

		if err := eh.Validate(); err != nil {
			return
		}
		_ = eh.Hash()
		_ = eh.LastHeader()
		_ = trusted.Verify(&eh)
		_ = adjacent.Verify(&eh)
	})
}
