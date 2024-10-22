package byzantine

import (
	"context"
	"hash"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mhcore "github.com/multiformats/go-multihash/core"
	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v3/test/util/malicious"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestBEFP_Validate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer t.Cleanup(cancel)
	bServ := ipld.NewMemBlockservice()

	byzSquare := edstest.RandByzantineEDS(t, 16)
	roots, err := share.NewAxisRoots(byzSquare)
	require.NoError(t, err)
	err = ipld.ImportEDS(ctx, byzSquare, bServ)
	require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData
	err = byzSquare.Repair(roots.RowRoots, roots.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)

	byzantine := NewErrByzantine(ctx, bServ.Blockstore(), roots, errRsmt2d)
	var errByz *ErrByzantine
	require.ErrorAs(t, byzantine, &errByz)

	proof := CreateBadEncodingProof([]byte("hash"), 0, errByz)
	befp, ok := proof.(*BadEncodingProof)
	require.True(t, ok)
	test := []struct {
		name           string
		prepareFn      func() error
		expectedResult func(error)
	}{
		{
			name: "valid BEFP",
			prepareFn: func() error {
				return proof.Validate(&header.ExtendedHeader{DAH: roots})
			},
			expectedResult: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "invalid BEFP for valid header",
			prepareFn: func() error {
				validSquare := edstest.RandEDS(t, 2)
				validRoots, err := share.NewAxisRoots(validSquare)
				require.NoError(t, err)
				err = ipld.ImportEDS(ctx, validSquare, bServ)
				require.NoError(t, err)
				validShares := validSquare.Flattened()
				errInvalidByz := NewErrByzantine(ctx, bServ.Blockstore(), validRoots,
					&rsmt2d.ErrByzantineData{
						Axis:   rsmt2d.Row,
						Index:  0,
						Shares: validShares[0:4],
					},
				)
				var errInvalid *ErrByzantine
				require.ErrorAs(t, errInvalidByz, &errInvalid)
				invalidBefp := CreateBadEncodingProof([]byte("hash"), 0, errInvalid)
				return invalidBefp.Validate(&header.ExtendedHeader{DAH: validRoots})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errNMTTreeRootsMatch)
			},
		},
		{
			name: "incorrect share with Proof",
			prepareFn: func() error {
				// break the first shareWithProof to test negative case
				sh, err := libshare.RandShares(2)
				require.NoError(t, err)
				nmtProof := nmt.NewInclusionProof(0, 1, nil, false)
				befp.Shares[0] = &ShareWithProof{sh[0], &nmtProof, rsmt2d.Row}
				return proof.Validate(&header.ExtendedHeader{DAH: roots})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectShare)
			},
		},
		{
			name: "invalid amount of shares",
			prepareFn: func() error {
				befp.Shares = befp.Shares[0 : len(befp.Shares)/2]
				return proof.Validate(&header.ExtendedHeader{DAH: roots})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectAmountOfShares)
			},
		},
		{
			name: "not enough shares to recompute the root",
			prepareFn: func() error {
				befp.Shares[0] = nil
				return proof.Validate(&header.ExtendedHeader{DAH: roots})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectAmountOfShares)
			},
		},
		{
			name: "index out of bounds",
			prepareFn: func() error {
				befp.Index = 100
				return proof.Validate(&header.ExtendedHeader{DAH: roots})
			},
			expectedResult: func(err error) {
				require.ErrorIs(t, err, errIncorrectIndex)
			},
		},
		{
			name: "heights mismatch",
			prepareFn: func() error {
				return proof.Validate(&header.ExtendedHeader{
					RawHeader: core.Header{
						Height: 42,
					},
					DAH: roots,
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
	shares, err := libshare.RandShares(squareSize * squareSize)
	require.NoError(t, err)
	eds, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	// get an arbitrary row
	rowIdx := squareSize / 2
	shareProofs := make([]*ShareWithProof, 0, eds.Width())
	for i := range shareProofs {
		proof, err := GetShareWithProof(ctx, bServ, roots, shares[i], rsmt2d.Row, rowIdx, i)
		require.NoError(t, err)
		shareProofs = append(shareProofs, proof)
	}

	// create a fake error for data that was encoded correctly
	fakeError := ErrByzantine{
		Index:  uint32(rowIdx),
		Shares: shareProofs,
		Axis:   rsmt2d.Row,
	}

	h := &header.ExtendedHeader{
		RawHeader: core.Header{
			Height: 420,
		},
		DAH: roots,
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	size := 4
	eds := edstest.RandEDS(t, size)

	shares := eds.Flattened()
	shares[0], shares[4] = shares[4], shares[0] // corrupting eds

	bServ := newNamespacedBlockService()
	batchAddr := ipld.NewNmtNodeAdder(ctx, bServ, ipld.MaxSizeBatchOption(size*2))

	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		malicious.NewConstructor(uint64(size), nmt.NodeVisitor(batchAddr.Visit)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")

	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	var errRsmt2d *rsmt2d.ErrByzantineData
	err = eds.Repair(roots.RowRoots, roots.ColumnRoots)
	require.ErrorAs(t, err, &errRsmt2d)

	err = batchAddr.Commit()
	require.NoError(t, err)

	byzantine := NewErrByzantine(ctx, bServ.Blockstore(), roots, errRsmt2d)
	var errByz *ErrByzantine
	require.ErrorAs(t, byzantine, &errByz)

	befp := CreateBadEncodingProof([]byte("hash"), 0, errByz)
	err = befp.Validate(&header.ExtendedHeader{DAH: roots})
	require.NoError(t, err)
}

// namespacedBlockService wraps `BlockService` and extends the verification part
// to avoid returning blocks that has out of order namespaces.
type namespacedBlockService struct {
	blockservice.BlockService
	// the data structure that is used on the networking level, in order
	// to verify the order of the namespaces
	prefix *cid.Prefix
}

func newNamespacedBlockService() *namespacedBlockService {
	sha256NamespaceFlagged := uint64(0x7701)
	// register the nmt hasher to validate the order of namespaces
	mhcore.Register(sha256NamespaceFlagged, func() hash.Hash {
		nh := nmt.NewNmtHasher(share.NewSHA256Hasher(), libshare.NamespaceSize, true)
		nh.Reset()
		return nh
	})

	bs := &namespacedBlockService{}
	bs.BlockService = ipld.NewMemBlockservice()

	bs.prefix = &cid.Prefix{
		Version: 1,
		Codec:   sha256NamespaceFlagged,
		MhType:  sha256NamespaceFlagged,
		// equals to NmtHasher.Size()
		MhLength: share.NewSHA256Hasher().Size() + 2*libshare.NamespaceSize,
	}
	return bs
}

func (n *namespacedBlockService) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	block, err := n.BlockService.GetBlock(ctx, c)
	if err != nil {
		return nil, err
	}

	_, err = n.prefix.Sum(block.RawData())
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (n *namespacedBlockService) GetBlocks(ctx context.Context, cids []cid.Cid) <-chan blocks.Block {
	blockCh := n.BlockService.GetBlocks(ctx, cids)
	resultCh := make(chan blocks.Block)

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(resultCh)
				return
			case block, ok := <-blockCh:
				if !ok {
					close(resultCh)
					return
				}
				if _, err := n.prefix.Sum(block.RawData()); err != nil {
					continue
				}
				resultCh <- block
			}
		}
	}()
	return resultCh
}
