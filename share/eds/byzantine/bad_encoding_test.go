package byzantine

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/test/util/malicious"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestBEFP_Validate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer t.Cleanup(cancel)
	bServ := ipld.NewMemBlockservice()

	square := edstest.RandByzantineEDS(t, 32)
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
	eds := edstest.RandEDS(t, 2)
	shares := eds.Flattened()
	squareSize := int(utils.SquareSize(len(shares)))

	shares[0], shares[1] = shares[1], shares[0] // corrupting eds
	bServ := ipld.NewMemBlockservice()
	batchAddr := ipld.NewNmtNodeAdder(context.Background(), bServ, ipld.MaxSizeBatchOption(squareSize/2))
	// create a malicious eds and store it in the blockservice
	// A tree that is created in `malicious.NewConstructor` will not check the
	// ordering of the namespaces and an error will not be returned on `Push`
	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		malicious.NewConstructor(2, nmt.NodeVisitor(batchAddr.Visit)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")

	str := hex.EncodeToString(shares[0][share.NamespaceSize:])
	t.Log("TESTS Shares0 ", str)
	t.Log("TESTS Shares1 ", hex.EncodeToString(shares[1][share.NamespaceSize:]))
	t.Log("TESTS Shares3 ", hex.EncodeToString(shares[3][share.NamespaceSize:]))
	t.Log("TESTS Shares4 ", hex.EncodeToString(shares[4][share.NamespaceSize:]))
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)

	_, err = eds.RowRoots()
	require.NoError(t, err)

	_, err = eds.ColRoots()
	require.NoError(t, err)

	err = batchAddr.Commit()
	require.NoError(t, err)

	shh0, _ := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.RowRoots[0]), 0, 4)
	shh1, _ := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.RowRoots[1]), 0, 4)

	t.Log("BSERV00 ", hex.EncodeToString(shh0.RawData()[share.NamespaceSize:]))
	t.Log("BSERV01 ", hex.EncodeToString(shh1.RawData()[share.NamespaceSize:]))

	shh0, _ = ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.RowRoots[0]), 1, 4)
	shh1, _ = ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.RowRoots[1]), 1, 4)

	t.Log("BSERV01 ", hex.EncodeToString(shh0.RawData()[share.NamespaceSize:]))
	t.Log("BSERV11 ", hex.EncodeToString(shh1.RawData()[share.NamespaceSize:]))

	//shares = eds.Flattened()
	//badShares := make([]share.Share, len(shares))
	//badShares[0], badShares[3], badShares[11], badShares[15] = shares[0], shares[3], shares[11], shares[15]
	//
	//// newEds ensures that we get errByzantineData with Axis = row
	//newEds, err := rsmt2d.ImportExtendedDataSquare(badShares, share.DefaultRSMT2DCodec(), malicious.NewConstructor(2))
	//require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData

	newEds, err := deepCopy(eds)
	require.NoError(t, err)

	err = newEds.Repair(dah.RowRoots, dah.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)
	t.Log(errRsmt2d)
	errByz := NewErrByzantine(context.Background(), bServ, &dah, errRsmt2d)
	t.Log("RSMT2D0 ", hex.EncodeToString(errRsmt2d.Shares[0][share.NamespaceSize:]))
	t.Log("RSMT2D1 ", hex.EncodeToString(errRsmt2d.Shares[1][share.NamespaceSize:]))
	require.NotNil(t, errByz)
	ns0, err := share.NamespaceFromBytes(shares[0][:share.NamespaceSize])
	require.NoError(t, err)

	ns3, err := share.NamespaceFromBytes(shares[3][:share.NamespaceSize])
	require.NoError(t, err)

	t.Log(ns0.String())
	t.Log(ns3.String())
	befp := CreateBadEncodingProof([]byte("hash"), 0, errByz)
	err = befp.Validate(&header.ExtendedHeader{DAH: &dah})
	assert.NoError(t, err)

}

func deepCopy(eds *rsmt2d.ExtendedDataSquare) (rsmt2d.ExtendedDataSquare, error) {
	imported, err := rsmt2d.ImportExtendedDataSquare(
		eds.Flattened(),
		share.DefaultRSMT2DCodec(),
		malicious.NewConstructor(2),
	)
	return *imported, err
}

func TestEds(t *testing.T) {
	eds := edstest.RandEDS(t, 2)
	shares := eds.Flattened()
	t.Log(len(shares))
	squareSize := int(utils.SquareSize(len(shares)))

	shares[0], shares[4] = shares[4], shares[0] // corrupting eds
	bServ := ipld.NewMemBlockservice()
	batchAddr := ipld.NewNmtNodeAdder(context.Background(), bServ, ipld.MaxSizeBatchOption(squareSize/2))
	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		malicious.NewConstructor(2, nmt.NodeVisitor(batchAddr.Visit)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")

	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)

	_, err = eds.RowRoots()
	require.NoError(t, err)

	_, err = eds.ColRoots()
	require.NoError(t, err)

	err = batchAddr.Commit()
	require.NoError(t, err)

	leaf0, err := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), 0, 4)
	leaf1, err := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), 1, 4)
	leaf2, err := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), 2, 4)
	leaf3, err := ipld.GetLeaf(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), 3, 4)

	proof0 := make([]cid.Cid, 0)
	proof1 := make([]cid.Cid, 0)
	proof2 := make([]cid.Cid, 0)
	proof3 := make([]cid.Cid, 0)

	proof0, err = ipld.GetProof(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), proof0, 0, 4)
	proof1, err = ipld.GetProof(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), proof1, 1, 4)
	proof2, err = ipld.GetProof(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), proof2, 2, 4)
	proof3, err = ipld.GetProof(context.Background(), bServ, ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]), proof3, 3, 4)

	shWP0 := NewShareWithProof(0, leaf0.RawData(), proof0)
	shWP1 := NewShareWithProof(1, leaf1.RawData(), proof1)
	shWP2 := NewShareWithProof(2, leaf2.RawData(), proof2)
	shWP3 := NewShareWithProof(3, leaf3.RawData(), proof3)

	ok0 := shWP0.Validate(ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]))
	ok1 := shWP1.Validate(ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]))
	ok2 := shWP2.Validate(ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]))
	ok3 := shWP3.Validate(ipld.MustCidFromNamespacedSha256(dah.ColumnRoots[0]))

	require.True(t, ok0)
	require.True(t, ok1)
	require.True(t, ok2)
	require.True(t, ok3)
	require.True(t, bytes.Equal(shares[0], leaf0.RawData()[29:]))

	treeCn := malicious.NewConstructor(2)
	tree := treeCn(rsmt2d.Row, 0)
	tree.Push(shares[0])
	tree.Push(shares[1])
	tree.Push(shares[2])
	tree.Push(shares[3])

	root, err := tree.Root()
	require.NoError(t, err)
	require.True(t, bytes.Equal(root, dah.RowRoots[0]))
}
