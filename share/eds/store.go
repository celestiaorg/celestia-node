package eds

import (
	"context"
	"os"

	"github.com/filecoin-project/dagstore/shard"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/ipfs/go-datastore"
)

const (
	blocksPath     = "/blocks/"
	indexPath      = "/index/"
	transientsPath = "/transients/"
)

type EDSStore struct { //nolint:revive
	dgstr  *dagstore.DAGStore
	mounts *mount.Registry

	topIdx index.Inverted
	carIdx index.FullIndexRepo

	basepath string
}

func NewEDSStore(basepath string, ds datastore.Batching) (*EDSStore, error) {
	r := mount.NewRegistry()
	err := r.Register("fs", &mount.FSMount{FS: os.DirFS(basepath + blocksPath)})

	if err != nil {
		return nil, err
	}

	fsRepo, err := index.NewFSRepo(basepath + indexPath)
	if err != nil {
		return nil, err
	}

	invertedRepo := index.NewInverted(ds)

	dagStore, err := dagstore.NewDAGStore(
		dagstore.Config{
			TransientsDir: basepath + transientsPath,
			IndexRepo:     fsRepo,
			Datastore:     ds,
			MountRegistry: r,
			TopLevelIndex: invertedRepo,
		},
	)
	if err != nil {
		return nil, err
	}

	return &EDSStore{
		basepath: basepath,
		dgstr:    dagStore,
		topIdx:   invertedRepo,
		carIdx:   fsRepo,
		mounts:   r,
	}, nil
}

// Put stores the given data square with DataRoot's hash as a key.
//
// The square is verified on the Exchange level, and Put only stores the square trusting it.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
// Additionally, the file gets indexed s.t. Store.Blockstore can access them.
func (s *EDSStore) Put(ctx context.Context, root share.Root, square *rsmt2d.ExtendedDataSquare) error {
	key := root.String()
	f, err := os.OpenFile(s.basepath+blocksPath+key, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	err = WriteEDS(ctx, square, f)
	if err != nil {
		return err
	}

	err = s.dgstr.RegisterShard(ctx, shard.KeyFromString(key), &mount.FSMount{
		FS:   os.DirFS(s.basepath + blocksPath),
		Path: key,
	}, nil, dagstore.RegisterOpts{})
	if err != nil {
		return err
	}
	return nil
}
