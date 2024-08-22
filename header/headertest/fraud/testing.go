package headerfraud

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/store"
)

// FraudMaker allows to produce an invalid header at the specified height in order to produce the
// BEFP.
type FraudMaker struct {
	t *testing.T

	vals   []types.PrivValidator
	valSet *types.ValidatorSet

	// height of the invalid header
	height int64

	prevHash bytes.HexBytes
}

func NewFraudMaker(t *testing.T, height int64, vals []types.PrivValidator, valSet *types.ValidatorSet) *FraudMaker {
	return &FraudMaker{
		t:      t,
		vals:   vals,
		valSet: valSet,
		height: height,
	}
}

func (f *FraudMaker) MakeExtendedHeader(odsSize int, edsStore *store.Store) header.ConstructFn {
	return func(
		h *types.Header,
		comm *types.Commit,
		vals *types.ValidatorSet,
		eds *rsmt2d.ExtendedDataSquare,
	) (*header.ExtendedHeader, error) {
		if h.Height < f.height {
			return header.MakeExtendedHeader(h, comm, vals, eds)
		}

		hdr := *h
		if h.Height == f.height {
			adder := ipld.NewProofsAdder(odsSize, false)
			square := edstest.RandByzantineEDS(f.t, odsSize, nmt.NodeVisitor(adder.VisitFn()))
			dah, err := da.NewDataAvailabilityHeader(square)
			require.NoError(f.t, err)
			hdr.DataHash = dah.Hash()

			ctx := ipld.CtxWithProofsAdder(context.Background(), adder)
			require.NoError(f.t, edsStore.Put(ctx, &dah, uint64(h.Height), square))

			*eds = *square
		}
		if h.Height > f.height {
			hdr.LastBlockID.Hash = f.prevHash
		}

		blockID := comm.BlockID
		blockID.Hash = hdr.Hash()
		voteSet := types.NewVoteSet(hdr.ChainID, hdr.Height, 0, tmproto.PrecommitType, f.valSet)
		commit, err := headertest.MakeCommit(blockID, hdr.Height, 0, voteSet, f.vals, time.Now())
		require.NoError(f.t, err)

		*h = hdr
		*comm = *commit
		f.prevHash = h.Hash()
		return header.MakeExtendedHeader(h, comm, vals, eds)
	}
}

func CreateFraudExtHeader(
	t *testing.T,
	eh *header.ExtendedHeader,
) *header.ExtendedHeader {
	square := edstest.RandByzantineEDS(t, len(eh.DAH.RowRoots))
	dah, err := da.NewDataAvailabilityHeader(square)
	require.NoError(t, err)
	eh.DAH = &dah
	eh.RawHeader.DataHash = dah.Hash()
	return eh
}
