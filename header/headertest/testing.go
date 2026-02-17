package headertest

import (
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/cometbft/cometbft/libs/bytes"
	tmrand "github.com/cometbft/cometbft/libs/rand"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v7/pkg/da"
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
	tail *header.ExtendedHeader

	// blockTime is optional - if set, the test suite will generate
	// blocks timestamped at the specified interval
	blockTime time.Duration
	startTime time.Time
}

// Option defines a functional option type for configuring TestSuite
type Option func(*TestSuite)

// WithBlockTime sets the block time duration for the test suite
func WithBlockTime(blockTime time.Duration) Option {
	return func(ts *TestSuite) {
		ts.blockTime = blockTime
	}
}

// WithStartTime sets the genesis start time for the test suite
func WithStartTime(startTime time.Time) Option {
	return func(ts *TestSuite) {
		ts.startTime = startTime
	}
}

// WithValidators sets the number of validators and their voting power
func WithValidators(numValidators int, votingPower int64) Option {
	return func(ts *TestSuite) {
		valSet, vals := RandValidatorSet(numValidators, votingPower)
		ts.vals = vals
		ts.valSet = valSet
	}
}

func NewStore(t *testing.T) libhead.Store[*header.ExtendedHeader] {
	return headertest.NewStore[*header.ExtendedHeader](t, NewTestSuite(t), 10)
}

func NewCustomStore(
	t *testing.T,
	generator headertest.Generator[*header.ExtendedHeader],
	numHeaders int,
) libhead.Store[*header.ExtendedHeader] {
	return headertest.NewStore[*header.ExtendedHeader](t, generator, numHeaders)
}

// NewTestSuite setups a new test suite with the provided options.
// If no options are provided, default values are used.
func NewTestSuite(t *testing.T, opts ...Option) *TestSuite {
	// Default: 3 validators, 10 voting power, no block time, current time
	valSet, vals := RandValidatorSet(3, 10)
	ts := &TestSuite{
		t:         t,
		vals:      vals,
		valSet:    valSet,
		blockTime: 0,
		startTime: time.Now(),
	}

	// Apply the provided options
	for _, opt := range opts {
		opt(ts)
	}

	return ts
}

// Legacy constructor for backwards compatibility
func NewTestSuiteWithNumValidators(t *testing.T, numValidators int, blockTime time.Duration) *TestSuite {
	return NewTestSuite(t,
		WithValidators(numValidators, 10),
		WithBlockTime(blockTime))
}

// Legacy constructor for backwards compatibility
func NewTestSuiteWithGenesisTime(t *testing.T, startTime time.Time, blockTime time.Duration) *TestSuite {
	return NewTestSuite(t,
		WithStartTime(startTime),
		WithBlockTime(blockTime))
}

func NewTestSuiteDefaults(t *testing.T) *TestSuite {
	valSet, vals := RandValidatorSet(3, 1)
	return &TestSuite{
		t:         t,
		vals:      vals,
		valSet:    valSet,
		blockTime: 1,
		startTime: time.Now(),
	}
}

func NewTestSuiteWithTail(t *testing.T, tail *header.ExtendedHeader) *TestSuite {
	valSet, vals := RandValidatorSet(3, 1)
	return &TestSuite{
		t:         t,
		vals:      vals,
		valSet:    valSet,
		blockTime: 1,
		startTime: time.Now(),
		tail:      tail,
	}
}

func (s *TestSuite) genesis() *header.ExtendedHeader {
	dah := share.EmptyEDSRoots()

	gen := RandRawHeader(s.t)

	gen.DataHash = dah.Hash()
	gen.ValidatorsHash = s.valSet.Hash()
	gen.NextValidatorsHash = s.valSet.Hash()
	gen.Height = 1
	gen.Time = s.startTime
	voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, s.valSet)
	blockID := RandBlockID(s.t)
	blockID.Hash = gen.Hash()
	commit, err := MakeCommit(blockID, gen.Height, 0, voteSet, s.vals, s.startTime)
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
	blockID types.BlockID,
	height int64,
	round int32,
	voteSet *types.VoteSet,
	validators []types.PrivValidator,
	now time.Time,
) (*types.Commit, error) {
	// all sign
	for i := range validators {
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
			return nil, fmt.Errorf("error signing vote: %w", err)
		}
	}

	return voteSet.MakeExtendedCommit(types.DefaultABCIParams()).ToCommit(), nil
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
		s.head = s.Tail()
	}
	return s.head
}

func (s *TestSuite) Tail() *header.ExtendedHeader {
	if s.tail == nil {
		s.tail = s.genesis()
	}
	return s.tail
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
		if s.tail == nil {
			return s.Head()
		}
		s.head = s.Tail()
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

	sigs := make([]tmproto.CommitSig, len(s.vals))
	for i, val := range s.vals {
		v := &types.Vote{
			ValidatorAddress: s.valSet.Validators[i].Address,
			ValidatorIndex:   int32(i),
			Height:           h.Height,
			Round:            round,
			Timestamp:        h.Time,
			Type:             tmproto.PrecommitType,
			BlockID:          bid,
		}
		sgntr, err := val.(types.MockPV).PrivKey.Sign(types.VoteSignBytes(h.ChainID, v.ToProto()))
		require.Nil(s.t, err)
		v.Signature = sgntr
		commitSig := v.CommitSig()
		sigs[i] = tmproto.CommitSig{
			BlockIdFlag:      tmproto.BlockIDFlag(commitSig.BlockIDFlag),
			ValidatorAddress: commitSig.ValidatorAddress,
			Timestamp:        commitSig.Timestamp,
			Signature:        commitSig.Signature,
		}
	}

	// Create a proto.Commit manually
	protoCommit := &tmproto.Commit{
		Height:     h.Height,
		Round:      round,
		BlockID:    bid.ToProto(),
		Signatures: sigs,
	}

	// Convert to types.Commit
	commit, err := types.CommitFromProto(protoCommit)
	require.NoError(s.t, err)

	return commit
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

// HeaderOption defines a functional option type for configuring ExtendedHeader
type HeaderOption func(*header.ExtendedHeader)

// WithTimestamp sets the timestamp for the extended header
func WithTimestamp(timestamp time.Time) HeaderOption {
	return func(eh *header.ExtendedHeader) {
		rawHeader := eh.RawHeader
		rawHeader.Time = timestamp
		eh.RawHeader = rawHeader
	}
}

// WithDAH sets the DataAvailabilityHeader for the extended header
func WithDAH(dah *da.DataAvailabilityHeader) HeaderOption {
	return func(eh *header.ExtendedHeader) {
		rawHeader := eh.RawHeader
		rawHeader.DataHash = dah.Hash()
		eh.RawHeader = rawHeader
		eh.DAH = dah
	}
}

// WithHeight sets the height for the extended header
func WithHeight(height int64) HeaderOption {
	return func(eh *header.ExtendedHeader) {
		rawHeader := eh.RawHeader
		rawHeader.Height = height
		eh.RawHeader = rawHeader
	}
}

// RandExtendedHeader provides an ExtendedHeader fixture with optional configurations.
func RandExtendedHeader(t testing.TB, opts ...HeaderOption) *header.ExtendedHeader {
	dah := share.EmptyEDSRoots()

	rh := RandRawHeader(t)
	rh.DataHash = dah.Hash()

	valSet, vals := RandValidatorSet(3, 1)
	rh.ValidatorsHash = valSet.Hash()
	voteSet := types.NewVoteSet(rh.ChainID, rh.Height, 0, tmproto.PrecommitType, valSet)
	blockID := RandBlockID(t)
	blockID.Hash = rh.Hash()
	commit, err := MakeCommit(blockID, rh.Height, 0, voteSet, vals, rh.Time)
	require.NoError(t, err)

	eh := &header.ExtendedHeader{
		RawHeader:    *rh,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          dah,
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(eh)
	}

	return eh
}

// Legacy function for backwards compatibility
func RandExtendedHeaderAtTimestamp(t testing.TB, timestamp time.Time) *header.ExtendedHeader {
	return RandExtendedHeader(t, WithTimestamp(timestamp))
}

// Legacy function for backwards compatibility
func RandExtendedHeaderWithRoot(t testing.TB, dah *da.DataAvailabilityHeader) *header.ExtendedHeader {
	return RandExtendedHeader(t, WithDAH(dah))
}

func RandValidatorSet(numValidators int, votingPower int64) (*types.ValidatorSet, []types.PrivValidator) {
	var (
		valz           = make([]*types.Validator, numValidators)
		privValidators = make([]types.PrivValidator, numValidators)
	)

	for i := range numValidators {
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

func NewSubscriber(
	t *testing.T,
	store libhead.Store[*header.ExtendedHeader],
	suite *TestSuite,
	num int,
) *headertest.Subscriber[*header.ExtendedHeader] {
	headers := suite.GenExtendedHeaders(num)
	err := store.Append(context.TODO(), headers...)
	require.NoError(t, err)
	return &headertest.Subscriber[*header.ExtendedHeader]{
		Headers: headers,
	}
}

var _ libhead.Subscriber[*header.ExtendedHeader] = &Subscriber{}
