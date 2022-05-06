// TODO(@Wondertan): Ideally, we should move that into subpackage, so this does not get included into binary of
//  production code, but that does not matter at the moment.
package header

import (
	"context"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/da"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// NewTestStore creates initialized and started in memory header Store which is useful for testing.
func NewTestStore(ctx context.Context, t *testing.T, head *ExtendedHeader) Store {
	store, err := NewStoreWithHead(ctx, sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := store.Stop(ctx)
		require.NoError(t, err)
	})
	return store
}

// TestSuite provides everything you need to test chain of Headers.
// If not, please don't hesitate to extend it for your case.
type TestSuite struct {
	t *testing.T

	vals    []types.PrivValidator
	valSet  *types.ValidatorSet
	valPntr int

	head *ExtendedHeader
}

// NewTestSuite setups a new test suite with a given number of validators.
func NewTestSuite(t *testing.T, num int) *TestSuite {
	valSet, vals := types.RandValidatorSet(num, 10)
	return &TestSuite{
		t:      t,
		vals:   vals,
		valSet: valSet,
	}
}

func (s *TestSuite) genesis() *ExtendedHeader {
	gen := RandRawHeader(s.t)
	gen.ValidatorsHash = s.valSet.Hash()
	gen.NextValidatorsHash = s.valSet.Hash()
	gen.Height = 1
	voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, s.valSet)
	commit, err := types.MakeCommit(RandBlockID(s.t), gen.Height, 0, voteSet, s.vals, time.Now())
	require.NoError(s.t, err)
	dah := EmptyDAH()
	eh := &ExtendedHeader{
		RawHeader:    *gen,
		Commit:       commit,
		ValidatorSet: s.valSet,
		DAH:          &dah,
	}
	require.NoError(s.t, eh.ValidateBasic())
	return eh
}

func (s *TestSuite) Head() *ExtendedHeader {
	if s.head == nil {
		s.head = s.genesis()
	}
	return s.head
}

func (s *TestSuite) GenExtendedHeaders(num int) []*ExtendedHeader {
	headers := make([]*ExtendedHeader, num)
	for i := range headers {
		headers[i] = s.GenExtendedHeader()
	}
	return headers
}

func (s *TestSuite) GenExtendedHeader() *ExtendedHeader {
	if s.head == nil {
		s.head = s.genesis()
		return s.head
	}

	dah := da.MinDataAvailabilityHeader()
	height := s.Head().Height + 1
	rh := s.GenRawHeader(height, s.Head().Hash(), s.Head().Commit.Hash(), dah.Hash())
	s.head = &ExtendedHeader{
		RawHeader:    *rh,
		Commit:       s.Commit(rh),
		ValidatorSet: s.valSet,
		DAH:          &dah,
	}
	require.NoError(s.t, s.head.ValidateBasic())
	return s.head
}

func (s *TestSuite) GenRawHeader(
	height int64, lastHeader, lastCommit, dataHash bytes.HexBytes) *RawHeader {
	rh := RandRawHeader(s.t)
	rh.Height = height
	rh.Time = time.Now()
	rh.LastBlockID = types.BlockID{Hash: lastHeader}
	rh.LastCommitHash = lastCommit
	rh.DataHash = dataHash
	rh.ValidatorsHash = s.valSet.Hash()
	rh.NextValidatorsHash = s.valSet.Hash()
	rh.ProposerAddress = s.nextProposer().Address
	return rh
}

func (s *TestSuite) Commit(h *RawHeader) *types.Commit {
	bid := types.BlockID{
		Hash: h.Hash(),
		// Unfortunately, we still have to commit PartSetHeader even we don't need it in Celestia
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: tmrand.Bytes(32)},
	}
	round := int32(0)
	comms := make([]types.CommitSig, len(s.vals))
	for i, val := range s.vals {
		v := &types.Vote{
			ValidatorAddress: s.valSet.Validators[i].Address,
			ValidatorIndex:   int32(i),
			Height:           h.Height,
			Round:            round,
			Timestamp:        tmtime.Now(),
			Type:             tmproto.PrecommitType,
			BlockID:          bid,
		}
		sgntr, err := val.(types.MockPV).PrivKey.Sign(types.VoteSignBytes(h.ChainID, v.ToProto()))
		require.Nil(s.t, err)
		v.Signature = sgntr
		comms[i] = v.CommitSig()
	}

	return types.NewCommit(h.Height, round, bid, comms)
}

func (s *TestSuite) nextProposer() *types.Validator {
	if s.valPntr == len(s.valSet.Validators)-1 {
		s.valPntr = 0
	} else {
		s.valPntr++
	}
	val := s.valSet.Validators[s.valPntr]
	return val
}

// RandExtendedHeader provides an ExtendedHeader fixture.
func RandExtendedHeader(t *testing.T) *ExtendedHeader {
	rh := RandRawHeader(t)
	valSet, vals := types.RandValidatorSet(3, 1)
	rh.ValidatorsHash = valSet.Hash()
	voteSet := types.NewVoteSet(rh.ChainID, rh.Height, 0, tmproto.PrecommitType, valSet)
	commit, err := types.MakeCommit(RandBlockID(t), rh.Height, 0, voteSet, vals, time.Now())
	require.NoError(t, err)
	dah := EmptyDAH()
	return &ExtendedHeader{
		RawHeader:    *rh,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          &dah,
	}
}

// RandRawHeader provides a RawHeader fixture.
func RandRawHeader(t *testing.T) *RawHeader {
	return &RawHeader{
		Version:            version.Consensus{Block: 11, App: 1},
		ChainID:            "test",
		Height:             mrand.Int63(), //nolint:gosec
		Time:               time.Now(),
		LastBlockID:        RandBlockID(t),
		LastCommitHash:     tmrand.Bytes(32),
		DataHash:           tmrand.Bytes(32),
		ValidatorsHash:     tmrand.Bytes(32),
		NextValidatorsHash: tmrand.Bytes(32),
		ConsensusHash:      tmrand.Bytes(32),
		AppHash:            tmrand.Bytes(32),
		LastResultsHash:    tmrand.Bytes(32),
		EvidenceHash:       tmhash.Sum([]byte{}),
		ProposerAddress:    tmrand.Bytes(20),
	}
}

// RandBlockID provides a BlockID fixture.
func RandBlockID(t *testing.T) types.BlockID {
	bid := types.BlockID{
		Hash: make([]byte, 32),
		PartSetHeader: types.PartSetHeader{
			Total: 123,
			Hash:  make([]byte, 32),
		},
	}
	mrand.Read(bid.Hash)               //nolint:gosec
	mrand.Read(bid.PartSetHeader.Hash) //nolint:gosec
	return bid
}

type DummySubscriber struct {
	Headers []*ExtendedHeader
}

func (mhs *DummySubscriber) AddValidator(Validator) error {
	return nil
}

func (mhs *DummySubscriber) Subscribe() (Subscription, error) {
	return mhs, nil
}

func (mhs *DummySubscriber) NextHeader(ctx context.Context) (*ExtendedHeader, error) {
	defer func() {
		if len(mhs.Headers) > 1 {
			// pop the already-returned header
			cp := mhs.Headers
			mhs.Headers = cp[1:]
		} else {
			mhs.Headers = make([]*ExtendedHeader, 0)
		}
	}()
	if len(mhs.Headers) == 0 {
		return nil, context.Canceled
	}
	return mhs.Headers[0], nil
}

func (mhs *DummySubscriber) Cancel() {}
