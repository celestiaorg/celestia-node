package headertest

import (
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/celestiaorg/celestia-app/pkg/da"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/headertest"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("headertest")

// TestSuite provides everything you need to test chain of Headers.
// If not, please don't hesitate to extend it for your case.
type TestSuite struct {
	t *testing.T

	vals    []types.PrivValidator
	valSet  *types.ValidatorSet
	valPntr int

	head *header.ExtendedHeader
}

func NewStore(t *testing.T) libhead.Store[*header.ExtendedHeader] {
	return headertest.NewStore[*header.ExtendedHeader](t, NewTestSuite(t, 3), 10)
}

// NewTestSuite setups a new test suite with a given number of validators.
func NewTestSuite(t *testing.T, num int) *TestSuite {
	valSet, vals := RandValidatorSet(num, 10)
	return &TestSuite{
		t:      t,
		vals:   vals,
		valSet: valSet,
	}
}

func (s *TestSuite) genesis() *header.ExtendedHeader {
	dah := header.EmptyDAH()

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
		DAH:          &dah,
	}
	require.NoError(s.t, eh.Validate())
	return eh
}

func MakeCommit(blockID types.BlockID, height int64, round int32,
	voteSet *types.VoteSet, validators []types.PrivValidator, now time.Time) (*types.Commit, error) {

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

	dah := da.MinDataAvailabilityHeader()
	height := s.Head().Height() + 1
	rh := s.GenRawHeader(height, s.Head().Hash(), libhead.Hash(s.Head().Commit.Hash()), dah.Hash())
	s.head = &header.ExtendedHeader{
		RawHeader:    *rh,
		Commit:       s.Commit(rh),
		ValidatorSet: s.valSet,
		DAH:          &dah,
	}
	require.NoError(s.t, s.head.Validate())
	return s.head
}

func (s *TestSuite) GenRawHeader(
	height int64, lastHeader, lastCommit, dataHash libhead.Hash) *header.RawHeader {
	rh := RandRawHeader(s.t)
	rh.Height = height
	rh.Time = time.Now()
	rh.LastBlockID = types.BlockID{Hash: bytes.HexBytes(lastHeader)}
	rh.LastCommitHash = bytes.HexBytes(lastCommit)
	rh.DataHash = bytes.HexBytes(dataHash)
	rh.ValidatorsHash = s.valSet.Hash()
	rh.NextValidatorsHash = s.valSet.Hash()
	rh.ProposerAddress = s.nextProposer().Address
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
func RandExtendedHeader(t *testing.T) *header.ExtendedHeader {
	dah := header.EmptyDAH()

	rh := RandRawHeader(t)
	rh.DataHash = dah.Hash()

	valSet, vals := RandValidatorSet(3, 1)
	rh.ValidatorsHash = valSet.Hash()
	voteSet := types.NewVoteSet(rh.ChainID, rh.Height, 0, tmproto.PrecommitType, valSet)
	blockID := RandBlockID(t)
	blockID.Hash = rh.Hash()
	commit, err := MakeCommit(blockID, rh.Height, 0, voteSet, vals, time.Now())
	require.NoError(t, err)

	return &header.ExtendedHeader{
		RawHeader:    *rh,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          &dah,
	}
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
func RandRawHeader(t *testing.T) *header.RawHeader {
	return &header.RawHeader{
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
func RandBlockID(*testing.T) types.BlockID {
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

// FraudMaker creates a custom ConstructFn that breaks the block at the given height.
func FraudMaker(t *testing.T, faultHeight int64, bServ blockservice.BlockService) header.ConstructFn {
	log.Warn("Corrupting block...", "height", faultHeight)
	return func(ctx context.Context,
		h *types.Header,
		comm *types.Commit,
		vals *types.ValidatorSet,
		eds *rsmt2d.ExtendedDataSquare,
	) (*header.ExtendedHeader, error) {
		if h.Height == faultHeight {
			eh := &header.ExtendedHeader{
				RawHeader:    *h,
				Commit:       comm,
				ValidatorSet: vals,
			}

			eh, dataSq := CreateFraudExtHeader(t, eh, bServ)
			if eds != nil {
				*eds = *dataSq
			}
			return eh, nil
		}
		return header.MakeExtendedHeader(ctx, h, comm, vals, eds)
	}
}

func ExtendedHeaderFromEDS(t *testing.T, height uint64, eds *rsmt2d.ExtendedDataSquare) *header.ExtendedHeader {
	valSet, vals := RandValidatorSet(10, 10)
	gen := RandRawHeader(t)
	dah := da.NewDataAvailabilityHeader(eds)

	gen.DataHash = dah.Hash()
	gen.ValidatorsHash = valSet.Hash()
	gen.NextValidatorsHash = valSet.Hash()
	gen.Height = int64(height)
	blockID := RandBlockID(t)
	blockID.Hash = gen.Hash()
	voteSet := types.NewVoteSet(gen.ChainID, gen.Height, 0, tmproto.PrecommitType, valSet)
	commit, err := MakeCommit(blockID, gen.Height, 0, voteSet, vals, time.Now())
	require.NoError(t, err)

	eh := &header.ExtendedHeader{
		RawHeader:    *gen,
		Commit:       commit,
		ValidatorSet: valSet,
		DAH:          &dah,
	}
	require.NoError(t, eh.Validate())
	return eh
}

func CreateFraudExtHeader(
	t *testing.T,
	eh *header.ExtendedHeader,
	serv blockservice.BlockService,
) (*header.ExtendedHeader, *rsmt2d.ExtendedDataSquare) {
	square := edstest.RandByzantineEDS(t, 16)
	err := ipld.ImportEDS(context.Background(), square, serv)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(square)
	eh.DAH = &dah
	eh.RawHeader.DataHash = dah.Hash()
	return eh, square
}

type Subscriber struct {
	headertest.Subscriber[*header.ExtendedHeader]
}

var _ libhead.Subscriber[*header.ExtendedHeader] = &Subscriber{}
