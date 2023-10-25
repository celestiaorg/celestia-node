package byzantine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/test/util/malicious"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestBEFP_Validate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer t.Cleanup(cancel)
	bServ := ipld.NewMemBlockservice()

	square := edstest.RandByzantineEDS(t, 16)
	dah, err := da.NewDataAvailabilityHeader(square)
	require.NoError(t, err)
	err = ipld.ImportEDS(ctx, square, bServ)
	require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData
	err = square.Repair(dah.RowRoots, dah.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)

	errByz := NewErrByzantine(ctx, bServ, &dah, errRsmt2d)

	befp := CreateBadEncodingProof([]byte("hash"), 0, errByz)

	var test = []struct {
		name           string
		prepareFn      func() error
		expectedResult func(error)
	}{
		{
			name: "valid BEFP",
			prepareFn: func() error {
				return befp.Validate(&header.ExtendedHeader{DAH: &dah})
			},
			expectedResult: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "invalid BEFP for valid header",
			prepareFn: func() error {
				validSquare := edstest.RandEDS(t, 2)
				validDah, err := da.NewDataAvailabilityHeader(validSquare)
				require.NoError(t, err)
				err = ipld.ImportEDS(ctx, validSquare, bServ)
				require.NoError(t, err)
				validShares := validSquare.Flattened()
				errInvalidByz := NewErrByzantine(ctx, bServ, &validDah,
					&rsmt2d.ErrByzantineData{
						Axis:   rsmt2d.Row,
						Index:  0,
						Shares: validShares[0:4],
					},
				)
				invalidBefp := CreateBadEncodingProof([]byte("hash"), 0, errInvalidByz)
				return invalidBefp.Validate(&header.ExtendedHeader{DAH: &validDah})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errNMTTreeRootsMatch)
			},
		},
		{
			name: "incorrect share with Proof",
			prepareFn: func() error {
				befp, ok := befp.(*BadEncodingProof)
				require.True(t, ok)
				befp.Shares[0].Share = befp.Shares[1].Share
				return befp.Validate(&header.ExtendedHeader{DAH: &dah})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectShare)
			},
		},
		{
			name: "invalid amount of shares",
			prepareFn: func() error {
				befp, ok := befp.(*BadEncodingProof)
				require.True(t, ok)
				befp.Shares = befp.Shares[0 : len(befp.Shares)/2]
				return befp.Validate(&header.ExtendedHeader{DAH: &dah})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectAmountOfShares)
			},
		},
		{
			name: "not enough shares to recompute the root",
			prepareFn: func() error {
				befp, ok := befp.(*BadEncodingProof)
				require.True(t, ok)
				befp.Shares[0] = nil
				return befp.Validate(&header.ExtendedHeader{DAH: &dah})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectAmountOfShares)
			},
		},
		{
			name: "index out of bounds",
			prepareFn: func() error {
				befp, ok := befp.(*BadEncodingProof)
				require.True(t, ok)
				befpCopy := *befp
				befpCopy.Index = 100
				return befpCopy.Validate(&header.ExtendedHeader{DAH: &dah})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectIndex)
			},
		},
		{
			name: "heights mismatch",
			prepareFn: func() error {
				return befp.Validate(&header.ExtendedHeader{
					RawHeader: core.Header{
						Height: 42,
					},
					DAH: &dah,
				})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errHeightMismatch)
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			err = tt.prepareFn()
			tt.expectedResult(err)
		})
	}
}

// TestIncorrectBadEncodingFraudProof asserts that BEFP is not generated for the correct data
func TestIncorrectBadEncodingFraudProof(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := ipld.NewMemBlockservice()

	squareSize := 8
	shares := sharetest.RandShares(t, squareSize*squareSize)

	eds, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

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
		DAH: dah,
		Commit: &core.Commit{
			BlockID: core.BlockID{
				Hash: []byte("made up hash"),
			},
		},
	}

	proof := CreateBadEncodingProof(h.Hash(), h.Height(), &fakeError)
	err = proof.Validate(h)
	require.Error(t, err)
}

func TestBEFP_ValidateOutOfOrderShares(t *testing.T) {
	// skipping it for now because `malicious` package has a small issue: Constructor does not apply
	// passed options, so it's not possible to store shares and thus get proofs for them.
	// should be ok once app team will fix it.
	t.Skip()
	eds := edstest.RandEDS(t, 16)
	shares := eds.Flattened()
	shares[0], shares[1] = shares[1], shares[0] // corrupting eds
	bServ := ipld.NewMemBlockservice()
	batchAddr := ipld.NewNmtNodeAdder(context.Background(), bServ, ipld.MaxSizeBatchOption(16*2))
	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		malicious.NewConstructor(16, nmt.NodeVisitor(batchAddr.Visit)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")

	err = batchAddr.Commit()
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData
	err = eds.Repair(dah.RowRoots, dah.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)

	errByz := NewErrByzantine(context.Background(), bServ, &dah, errRsmt2d)

	befp := CreateBadEncodingProof([]byte("hash"), 0, errByz)
	err = befp.Validate(&header.ExtendedHeader{DAH: &dah})
	require.Error(t, err)
}
