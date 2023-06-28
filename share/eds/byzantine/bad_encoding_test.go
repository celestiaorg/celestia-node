package byzantine

import (
	"context"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestBadEncodingFraudProof(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	square := edstest.RandByzantineEDS(t, 16)
	dah := da.NewDataAvailabilityHeader(square)
	err := ipld.ImportEDS(ctx, square, bServ)
	require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData
	err = square.Repair(dah.RowRoots, dah.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)

	errByz := NewErrByzantine(ctx, bServ, &dah, errRsmt2d)

	befp := CreateBadEncodingProof([]byte("hash"), 0, errByz)
	err = befp.Validate(&header.ExtendedHeader{
		DAH: &dah,
	})
	assert.NoError(t, err)
}

// TestIncorrectBadEncodingFraudProof asserts that BEFP is not generated for the correct data
func TestIncorrectBadEncodingFraudProof(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := mdutils.Bserv()

	squareSize := 8
	shares := sharetest.RandShares(t, squareSize*squareSize)

	eds, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(eds)

	// get an arbitrary row
	row := uint(squareSize / 2)
	rowShares := eds.Row(row)
	rowRoot := dah.RowRoots[row]

	shareProofs, err := GetProofsForShares(ctx, bServ, ipld.MustCidFromNamespacedSha256(rowRoot), rowShares)
	require.NoError(t, err)

	// create a fake error for data that was encoded correctly
	fakeError := ErrByzantine{
		Index:  uint32(row),
		Shares: shareProofs,
		Axis:   rsmt2d.Row,
	}

	h := &header.ExtendedHeader{
		RawHeader: core.Header{
			Height: 420,
		},
		DAH: &dah,
		Commit: &core.Commit{
			BlockID: core.BlockID{
				Hash: []byte("made up hash"),
			},
		},
	}

	proof := CreateBadEncodingProof(h.Hash(), uint64(h.Height()), &fakeError)
	err = proof.Validate(h)
	require.Error(t, err)
}
