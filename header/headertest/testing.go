package headertest

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/headertest"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

// TestSuite provides everything you need to test chain of Headers.
// If not, please don't hesitate to extend it for your case.
type TestSuite struct {
	t *testing.T

	vals    []types.PrivValidator
	valSet  *types.ValidatorSet
	valPntr int

	head *header.ExtendedHeader

	// blockTime is optional - if set, the test suite will generate
	// blocks timestamped at the specified interval
	blockTime time.Duration
}

func NewStore(t *testing.T) libhead.Store[*header.ExtendedHeader] {
	return headertest.NewStore[*header.ExtendedHeader](t, NewTestSuite(t, 3, 0), 10)
}

func NewCustomStore(
	t *testing.T,
	generator headertest.Generator[*header.ExtendedHeader],
	numHeaders int,
) libhead.Store[*header.ExtendedHeader] {
	return headertest.NewStore[*header.ExtendedHeader](t, generator, numHeaders)
}

// NewTestSuite setups a new test suite with a given number of validators.
func NewTestSuite(t *testing.T, numValidators int, blockTime time.Duration) *TestSuite {
	valSet, vals := RandValidatorSet(numValidators, 10)
	return &TestSuite{
		t:         t,
		vals:      vals,
		valSet:    valSet,
		blockTime: blockTime,
	}
}

func (s *TestSuite) genesis() *header.ExtendedHeader {
	dah := share.EmptyEDSRoots()

	gen := RandRawHeader(s.t)

	gen.DataHash = dah.Hash()
	gen.ValidatorsHash = s.valSet.Hash()
	gen.NextValidatorsHash = s.valSet.Hash()
	gen.Height = 1
	voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, s.valSet)
	blockID := RandBlockID(s.t)
	blockID.Hash = gen.Hash()
	commit, err := MakeCommit(blockID, gen.Height, 0, voteSet, s.vals, time.Now())
	require.NoError(s.t, err)

	eh := &header.ExtendedHeader{
		RawHeader:    *gen,
		Commit:       commit,
		ValidatorSet: s.valSet,
		DAH:          dah,
	}
	require.NoError(s.t, eh.Validate())
	return eh
}

func MakeCommit(
	blockID types.BlockID, height int64, round int32,
	voteSet *types.VoteSet, validators []types.PrivValidator, now time.Time,
) (*types.Commit, error) {
	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey()
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &types.Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal types.PrivValidator, vote *types.Vote, voteSet *types.VoteSet) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature
	return voteSet.AddVote(vote)
}

func (s *TestSuite) Head() *header.ExtendedHeader {
	if s.head == nil {
		s.head = s.genesis()
	}
	return s.head
}

func (s *TestSuite) GenExtendedHeaders(num int) []*header.ExtendedHeader {
	headers := make([]*header.ExtendedHeader, num)
	for i := range headers {
		headers[i] = s.NextHeader()
	}
	return headers
}

var _ headertest.Generator[*header.ExtendedHeader] = &TestSuite{}

func (s *TestSuite) NextHeader() *header.ExtendedHeader {
	if s.head == nil {
		s.head = s.genesis()
		return s.head
	}

	dah := share.EmptyEDSRoots()
	height := s.Head().Height() + 1
	rh := s.GenRawHeader(height, s.Head().Hash(), libhead.Hash(s.Head().Commit.Hash()), dah.Hash())
	s.head = &header.ExtendedHeader{
		RawHeader:    *rh,
		Commit:       s.Commit(rh),
		ValidatorSet: s.valSet,
		DAH:          dah,
	}
	require.NoError(s.t, s.head.Validate())
	return s.head
}

func (s *TestSuite) GenRawHeader(
	height uint64, lastHeader, lastCommit, dataHash libhead.Hash,
) *header.RawHeader {
	rh := RandRawHeader(s.t)
	rh.Height = int64(height)
	rh.LastBlockID = types.BlockID{Hash: bytes.HexBytes(lastHeader)}
	rh.LastCommitHash = bytes.HexBytes(lastCommit)
	rh.DataHash = bytes.HexBytes(dataHash)
	rh.ValidatorsHash = s.valSet.Hash()
	rh.NextValidatorsHash = s.valSet.Hash()
	rh.ProposerAddress = s.nextProposer().Address

	rh.Time = time.Now().UTC()
	if s.blockTime > 0 {
		rh.Time = s.Head().Time().UTC().Add(s.blockTime)
	}

	return rh
}

func (s *TestSuite) Commit(h *header.RawHeader) *types.Commit {
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
			Timestamp:        tmtime.Now().UTC(),
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
func RandExtendedHeader(t testing.TB) *header.ExtendedHeader {
	timestamp := time.Now().UTC()
	return RandExtendedHeaderAtTimestamp(t, timestamp)
}

func RandExtendedHeaderAtTimestamp(t testing.TB, timestamp time.Time) *header.ExtendedHeader {
	dah := share.EmptyEDSRoots()

	rh := RandRawHeader(t)
	rh.DataHash = dah.Hash()

	valSet, vals := RandValidatorSet(3, 1)
	rh.ValidatorsHash = valSet.Hash()
	voteSet := types.NewVoteSet(rh.ChainID, rh.Height, 0, tmproto.PrecommitType, valSet)
	blockID := RandBlockID(t)
	blockID.Hash = rh.Hash()
	commit, err := MakeCommit(blockID, rh.Height, 0, voteSet, vals, timestamp)
	require.NoError(t, err)

	return &header.ExtendedHeader{
		RawHeader:    *rh,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          dah,
	}
}

func RandExtendedHeaderWithRoot(t testing.TB, dah *da.DataAvailabilityHeader) *header.ExtendedHeader {
	h := RandExtendedHeader(t)
	h.DataHash = dah.Hash()
	h.DAH = dah
	return h
}

func RandValidatorSet(numValidators int, votingPower int64) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return types.NewValidatorSet(valz), privValidators
}

func RandValidator(randPower bool, minPower int64) (*types.Validator, types.PrivValidator) {
	privVal := types.NewMockPV()
	votePower := minPower
	if randPower {
		votePower += int64(mrand.Uint32()) //nolint:gosec
	}
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := types.NewValidator(pubKey, votePower)
	return val, privVal
}

// RandRawHeader provides a RawHeader fixture.
func RandRawHeader(t testing.TB) *header.RawHeader {
	return &header.RawHeader{
		Version:            version.Consensus{Block: 11, App: 1},
		ChainID:            "test",
		Height:             mrand.Int63(), //nolint:gosec
		Time:               time.Now().UTC(),
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
func RandBlockID(testing.TB) types.BlockID {
	bid := types.BlockID{
		Hash: make([]byte, 32),
		PartSetHeader: types.PartSetHeader{
			Total: 123,
			Hash:  make([]byte, 32),
		},
	}
	_, _ = rand.Read(bid.Hash)
	_, _ = rand.Read(bid.PartSetHeader.Hash)
	return bid
}

func ExtendedHeadersFromEdsses(t testing.TB, edsses []*rsmt2d.ExtendedDataSquare) []*header.ExtendedHeader {
	valSet, vals := RandValidatorSet(10, 10)
	headers := make([]*header.ExtendedHeader, len(edsses))
	for i, eds := range edsses {
		gen := RandRawHeader(t)

		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		gen.DataHash = roots.Hash()
		gen.ValidatorsHash = valSet.Hash()
		gen.NextValidatorsHash = valSet.Hash()
		gen.Height = int64(i + 1)
		if i > 0 {
			gen.LastBlockID = headers[i-1].Commit.BlockID
			gen.LastCommitHash = headers[i-1].Commit.Hash()
		}
		blockID := RandBlockID(t)
		blockID.Hash = gen.Hash()
		voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, valSet)
		commit, err := MakeCommit(blockID, gen.Height, 0, voteSet, vals, time.Now().UTC())
		require.NoError(t, err)
		eh := &header.ExtendedHeader{
			RawHeader:    *gen,
			Commit:       commit,
			ValidatorSet: valSet,
			DAH:          roots,
		}
		require.NoError(t, eh.Validate())
		headers[i] = eh
	}
	return headers
}

func ExtendedHeaderFromEDS(t testing.TB, height uint64, eds *rsmt2d.ExtendedDataSquare) *header.ExtendedHeader {
	valSet, vals := RandValidatorSet(10, 10)
	gen := RandRawHeader(t)
	dah, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	gen.DataHash = dah.Hash()
	gen.ValidatorsHash = valSet.Hash()
	gen.NextValidatorsHash = valSet.Hash()
	gen.Height = int64(height)
	blockID := RandBlockID(t)
	blockID.Hash = gen.Hash()
	voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, valSet)
	commit, err := MakeCommit(blockID, gen.Height, 0, voteSet, vals, time.Now().UTC())
	require.NoError(t, err)

	eh := &header.ExtendedHeader{
		RawHeader:    *gen,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          dah,
	}
	require.NoError(t, eh.Validate())
	return eh
}

type Subscriber struct {
	headertest.Subscriber[*header.ExtendedHeader]
}

var _ libhead.Subscriber[*header.ExtendedHeader] = &Subscriber{}
