//go:build !race

package byzantine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
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

	byzantine := NewErrByzantine(ctx, bServ, &dah, errRsmt2d)
	var errByz *ErrByzantine
	require.ErrorAs(t, byzantine, &errByz)

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
				var errInvalid *ErrByzantine
				require.ErrorAs(t, errInvalidByz, &errInvalid)
				invalidBefp := CreateBadEncodingProof([]byte("hash"), 0, errInvalid)
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
