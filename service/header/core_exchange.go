package header

import (
	"bytes"
	"context"
	"fmt"
	"math"

	format "github.com/ipfs/go-ipld-format"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/ipld"
)

type CoreExchange struct {
	fetcher    *core.BlockFetcher
	shareStore format.DAGService
}

func NewCoreExchange(fetcher *core.BlockFetcher, dag format.DAGService) *CoreExchange {
	return &CoreExchange{
		fetcher:    fetcher,
		shareStore: dag,
	}
}

func (ce *CoreExchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	log.Debugw("core: requesting header", "height", height)
	intHeight := int64(height)
	block, err := ce.fetcher.GetBlock(ctx, &intHeight)
	if err != nil {
		return nil, err
	}
	return ce.generateExtendedHeaderFromBlock(block)
}

func (ce *CoreExchange) RequestHeaders(ctx context.Context, from, amount uint64) ([]*ExtendedHeader, error) {
	log.Debugw("core: requesting headers", "from", from, "to", from+amount)
	headers := make([]*ExtendedHeader, amount)
	for i := range headers {
		extHeader, err := ce.RequestHeader(ctx, from+uint64(i))
		if err != nil {
			return nil, err
		}

		headers[i] = extHeader
	}

	return headers, nil
}

func (ce *CoreExchange) RequestByHash(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	log.Debugw("core: requesting header", "hash", hash.String())
	block, err := ce.fetcher.GetBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	extHeader, err := ce.generateExtendedHeaderFromBlock(block)
	if err != nil {
		return nil, err
	}
	// verify hashes match
	if !hashMatch(hash, extHeader.Hash().Bytes()) {
		return nil, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, extHeader.Hash().Bytes())
	}
	return extHeader, nil
}

func hashMatch(expected, got []byte) bool {
	return bytes.Equal(expected, got)
}

func (ce *CoreExchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	log.Debug("core: requesting head")
	chainHead, err := ce.fetcher.GetBlock(ctx, nil)
	if err != nil {
		return nil, err
	}
	return ce.generateExtendedHeaderFromBlock(chainHead)
}

func (ce *CoreExchange) generateExtendedHeaderFromBlock(block *types.Block) (*ExtendedHeader, error) {
	// erasure code the block
	extended, err := ce.extendBlockData(block)
	if err != nil {
		log.Errorw("computing extended data square", "err msg", err, "block height",
			block.Height, "block hash", block.Hash().String())
		return nil, err
	}
	// write block data to store
	dah := da.NewDataAvailabilityHeader(extended)
	log.Debugw("generated DataAvailabilityHeader", "data root", dah.Hash())
	// create ExtendedHeader
	commit, err := ce.fetcher.Commit(context.Background(), &block.Height)
	if err != nil {
		log.Errorw("fetching commit", "err", err, "height", block.Height)
		return nil, err
	}

	valSet, err := ce.fetcher.ValidatorSet(context.Background(), &block.Height)
	if err != nil {
		log.Errorw("fetching validator set", "err", err, "height", block.Height)
		return nil, err
	}
	extHeader := &ExtendedHeader{
		RawHeader:    block.Header,
		DAH:          &dah,
		Commit:       commit,
		ValidatorSet: valSet,
	}
	// sanity check generated ExtendedHeader
	err = extHeader.ValidateBasic()
	if err != nil {
		return nil, err
	}
	return extHeader, nil
}

// extendBlockData erasure codes the given raw block's data and returns the
// erasure coded block data upon success.
// TODO @renaynay: this is duplicate code for purpose of DEVNET, delete post devnet
func (ce *CoreExchange) extendBlockData(raw *types.Block) (*rsmt2d.ExtendedDataSquare, error) {
	namespacedShares, _ := raw.Data.ComputeShares()
	shares := namespacedShares.RawShares()

	// create nmt adder wrapping batch adder
	batchAdder := ipld.NewNmtNodeAdder(context.Background(), format.NewBatch(context.Background(), ce.shareStore))

	// create the nmt wrapper to generate row and col commitments
	squareSize := squareSize64(len(namespacedShares))
	tree := wrapper.NewErasuredNamespacedMerkleTree(squareSize, nmt.NodeVisitor(batchAdder.Visit))

	// compute extended square
	return rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
}

// squareSize64 computes the square size as a uint64 from
// the given length of shares.
// TODO @renaynay: this is duplicate code for purpose of DEVNET, delete post devnet
func squareSize64(length int) uint64 {
	return uint64(math.Sqrt(float64(length)))
}
