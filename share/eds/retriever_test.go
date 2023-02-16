package eds

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func init() {
	// randomize quadrant fetching, otherwise quadrant sampling is deterministic
	rand.Seed(time.Now().UnixNano())
}

func TestRetriever_Retrieve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := mdutils.Bserv()
	r := NewRetriever(bServ)

	type test struct {
		name       string
		squareSize int
	}
	tests := []test{
		{"1x1(min)", 1},
		{"2x2(med)", 2},
		{"4x4(med)", 4},
		{"8x8(med)", 8},
		{"16x16(med)", 16},
		{"32x32(med)", 32},
		{"64x64(med)", 64},
		{"128x128(max)", share.MaxSquareSize},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// generate EDS
			shares := share.RandShares(t, tc.squareSize*tc.squareSize)
			in, err := share.AddShares(ctx, shares, bServ)
			require.NoError(t, err)

			// limit with timeout, specifically retrieval
			ctx, cancel := context.WithTimeout(ctx, time.Minute*5) // the timeout is big for the max size which is long
			defer cancel()

			dah := da.NewDataAvailabilityHeader(in)
			out, err := r.Retrieve(ctx, &dah)
			require.NoError(t, err)
			assert.True(t, share.EqualEDS(in, out))
		})
	}
}

func TestRetriever_ByzantineError(t *testing.T) {
	const width = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bserv := mdutils.Bserv()
	shares := share.ExtractEDS(share.RandEDS(t, width))
	_, err := share.ImportShares(ctx, shares, bserv)
	require.NoError(t, err)

	// corrupt shares so that eds erasure coding does not match
	copy(shares[14][8:], shares[15][8:])

	// import corrupted eds
	batchAdder := ipld.NewNmtNodeAdder(ctx, bserv, ipld.MaxSizeBatchOption(width*2))
	attackerEDS, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(width),
			nmt.NodeVisitor(batchAdder.Visit)),
	)
	require.NoError(t, err)
	err = batchAdder.Commit()
	require.NoError(t, err)

	// ensure we rcv an error
	dah := da.NewDataAvailabilityHeader(attackerEDS)
	r := NewRetriever(bserv)
	_, err = r.Retrieve(ctx, &dah)
	var errByz *byzantine.ErrByzantine
	require.ErrorAs(t, err, &errByz)
}

// TestRetriever_MultipleRandQuadrants asserts that reconstruction succeeds
// when any three random quadrants requested.
func TestRetriever_MultipleRandQuadrants(t *testing.T) {
	RetrieveQuadrantTimeout = time.Millisecond * 500
	const squareSize = 32
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	bServ := mdutils.Bserv()
	r := NewRetriever(bServ)

	// generate EDS
	shares := share.RandShares(t, squareSize*squareSize)
	in, err := share.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(in)
	ses, err := r.newSession(ctx, &dah)
	require.NoError(t, err)

	// wait until two additional quadrants requested
	// this reliably allows us to reproduce the issue
	time.Sleep(RetrieveQuadrantTimeout * 2)
	// then ensure we have enough shares for reconstruction for slow machines e.g. CI
	<-ses.Done()

	_, err = ses.Reconstruct(ctx)
	assert.NoError(t, err)
}

func TestFraudProofValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	var errByz *byzantine.ErrByzantine
	faultHeader, err := generateByzantineError(ctx, t, bServ)
	require.True(t, errors.As(err, &errByz))

	p := byzantine.CreateBadEncodingProof([]byte("hash"), uint64(faultHeader.Height()), errByz)
	err = p.Validate(faultHeader)
	require.NoError(t, err)
}

func generateByzantineError(
	ctx context.Context,
	t *testing.T,
	bServ blockservice.BlockService,
) (*header.ExtendedHeader, error) {
	_, store := fraud.CreateTestService(t, false)
	h, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	faultHeader, _ := headertest.CreateFraudExtHeader(t, h, bServ)
	_, err = NewRetriever(bServ).Retrieve(ctx, faultHeader.DAH)
	return faultHeader, err
}
