package share

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/ipld/plugin"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// WriteEDS writes the whole EDS into the given io.Writer as CARv1 file.
// All its shares and recomputed NMT proofs.
func WriteEDS(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, w io.Writer) error {
	// 1. Reimport EDS. This is needed to get the proofs.
	//    - Using Blockservice w/ offline exchange and in-memory blockstore.
	//    - With NodeVisitor, which saves ONLY PROOFS to the blockstore
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	store := blockstore.NewBlockstore(dstore)
	bs := blockservice.New(store, nil)
	shares := ipld.ExtractEDS(eds)
	if len(shares) == 0 {
		return fmt.Errorf("ipld: importing empty data")
	}
	squareSize := int(math.Sqrt(float64(len(shares))))
	// todo: batch size here
	batchAdder := ipld.NewNmtNodeAdder(ctx, bs, format.MaxSizeBatchOption(squareSize))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize/2), nmt.NodeVisitor(batchAdder.VisitInnerNodes))
	eds, err := rsmt2d.ImportExtendedDataSquare(shares, ipld.DefaultRSMT2DCodec(), tree.Constructor)
	if err != nil {
		return fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	eds.ColRoots()
	// commit the batch to DAG
	err = batchAdder.Commit()
	if err != nil {
		return fmt.Errorf("failure to commit the inner nodes to the dag: %w", err)
	}

	// 2. Creates and writes Carv1Header
	//    - Roots are the eds Row + Col roots
	rootCids, err := rootsToCids(eds)
	if err != nil {
		return fmt.Errorf("failure to get root cids: %w", err)
	}
	err = car.WriteHeader(&car.CarHeader{
		Roots:   rootCids,
		Version: 1,
	}, w)
	if err != nil {
		return fmt.Errorf("failure to write carv1 header: %w", err)
	}

	// 3. Iterates over shares in quadrant order vis eds.GetCell
	//    - Writes the shares in row-by-row order
	shares, err = quadrantOrder(eds)
	fmt.Println(shares)
	if err != nil {
		return fmt.Errorf("failure to get shares in quadrant order: %w", err)
	}
	for _, share := range shares {
		cid, err := plugin.CidFromNamespacedSha256(nmt.Sha256Namespace8FlaggedLeaf(share))
		if err != nil {
			return fmt.Errorf("failure to get cid from share: %w", err)
		}
		err = util.LdWrite(w, cid.Bytes(), share)
		if err != nil {
			return fmt.Errorf("failure to write share: %w", err)
		}
	}

	// 4. Iterates over in-memory Blockstore and writes proofs to the CAR
	proofs, err := store.AllKeysChan(ctx)
	if err != nil {
		return fmt.Errorf("failure to get all keys from the blockstore: %w", err)
	}
	for proofCid := range proofs {
		node, err := store.Get(ctx, proofCid)
		if err != nil {
			return fmt.Errorf("failure to get proof from the blockstore: %w", err)
		}
		err = util.LdWrite(w, proofCid.Bytes(), node.RawData())
		if err != nil {
			return fmt.Errorf("failure to write proof to the car: %w", err)
		}
	}

	return nil
}

func quadrantOrder(eds *rsmt2d.ExtendedDataSquare) ([][]byte, error) {
	size := eds.Width() * eds.Width()
	shares := make([][]byte, size)

	rowCount := eds.Width() / 2
	// TODO: Simplify this loop
	for i := 0; i < int(rowCount); i++ {
		for j := 0; j < int(rowCount); j++ {
			shares[(0*int(eds.Width()))+i*int(rowCount)+j] = eds.GetCell(uint(i), uint(j))
			shares[(1*int(eds.Width()))+i*int(rowCount)+j] = eds.GetCell(uint(i), uint(j)+rowCount)
			shares[(2*int(eds.Width()))+i*int(rowCount)+j] = eds.GetCell(uint(i)+rowCount, uint(j))
			shares[(3*int(eds.Width()))+i*int(rowCount)+j] = eds.GetCell(uint(i)+rowCount, uint(j)+rowCount)
		}
	}
	return shares, nil
}

func rootsToCids(eds *rsmt2d.ExtendedDataSquare) ([]cid.Cid, error) {
	var err error
	roots := append(eds.RowRoots(), eds.ColRoots()...)
	rootCids := make([]cid.Cid, len(roots))
	for i, r := range roots {
		rootCids[i], err = plugin.CidFromNamespacedSha256(r)
		if err != nil {
			return nil, fmt.Errorf("failure to get cid from root: %w", err)
		}
	}
	return rootCids, nil
}
