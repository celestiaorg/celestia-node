package eds

import (
	"context"
	"io"
	"os"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/rsmt2d"
)

const (
	blocksPath     = "/blocks/"
	indexPath      = "/index/"
	transientsPath = "/transients/"
)

type EDSStore struct { //nolint:revive
	dgstr  *dagstore.DAGStore
	bs     blockstore.Blockstore
	mounts *mount.Registry

	topIdx index.Inverted
	carIdx index.FullIndexRepo

	basepath string
}

func NewEDSStore(basepath string, ds datastore.Batching) (*EDSStore, error) {
	err := setupPath(basepath)
	if err != nil {
		return nil, err
	}

	r := mount.NewRegistry()
	err = r.Register("fs", &mount.FSMount{FS: os.DirFS(basepath + blocksPath)})

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

	s := &EDSStore{
		basepath: basepath,
		dgstr:    dagStore,
		topIdx:   invertedRepo,
		carIdx:   fsRepo,
		mounts:   r,
	}

	s.bs, err = NewEDSBlockstore(s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *EDSStore) Start(ctx context.Context) error {
	return s.dgstr.Start(ctx)
}

func (s *EDSStore) Stop() error {
	return s.dgstr.Close()
}

// Put stores the given data square with DataRoot's hash as a key.
//
// The square is verified on the Exchange level, and Put only stores the square trusting it.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
// Additionally, the file gets indexed s.t. store.Blockstore can access them.
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

	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.RegisterShard(ctx, shard.KeyFromString(key), &mount.FSMount{
		FS:   os.DirFS(s.basepath + blocksPath),
		Path: key,
	}, ch, dagstore.RegisterOpts{})

	if err != nil {
		return err
	}

	result := <-ch
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetCAR takes a DataRoot and returns a buffered reader to the respective EDS serialized as a CARv1 file.
//
// The Reader strictly reads the first quadrant(1/4) of EDS, omitting all the NMT Merkle proofs.
// Integrity of the store data is not verified.
//
// Caller must Close returned reader after reading.
func (s *EDSStore) GetCAR(ctx context.Context, root share.Root) (io.ReadCloser, error) {
	key := root.String()

	ch := make(chan dagstore.ShardResult, 1)
	err := s.dgstr.AcquireShard(ctx, shard.KeyFromString(key), ch, dagstore.AcquireOpts{})
	if err != nil {
		return nil, err
	}

	result := <-ch
	if result.Error != nil {
		return nil, result.Error
	}
	return result.Accessor, nil
}

// Blockstore returns an IPFS Blockstore providing access to individual shares/nodes of all EDS
// registered on the Store. NOTE: The Blockstore does not store whole Celestia Blocks but IPFS blocks.
// We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes so Bitswap can access those.
func (s *EDSStore) Blockstore() blockstore.Blockstore {
	return s.bs
}

// Remove removes EDS from Store by the given share.Root and cleans up all the indexing.
func (s *EDSStore) Remove(ctx context.Context, root share.Root) error {
	key := root.String()
	ch := make(chan dagstore.ShardResult, 1)
	err := s.dgstr.DestroyShard(ctx, shard.KeyFromString(key), ch, dagstore.DestroyOpts{})
	if err != nil {
		return err
	}

	result := <-ch
	if result.Error != nil {
		return result.Error
	}

	dropped, err := s.carIdx.DropFullIndex(shard.KeyFromString(key))
	if !dropped {
		log.Warnf("failed to drop index for %s", key)
	}
	return err
}

// Get reads EDS out of Store by given DataRoot.
//
// It reads only one quadrant(1/4) of the EDS and verifies the integrity of the stored data by recomputing it.
func (s *EDSStore) Get(ctx context.Context, root share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	key := root.String()
	f, err := os.OpenFile(s.basepath+blocksPath+key, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	return ReadEDS(ctx, f, root)
}

// Has checks if EDS exists by the given share.Root.
func (s *EDSStore) Has(ctx context.Context, root share.Root) (bool, error) {
	key := root.String()
	info, err := s.dgstr.GetShardInfo(shard.KeyFromString(key))
	if err == dagstore.ErrShardUnknown {
		return false, err
	}

	return true, info.Error
}

func setupPath(basepath string) error {
	err := os.Mkdir(basepath+blocksPath, 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(basepath+transientsPath, 0755)
	if err != nil {
		return err
	}
	return os.Mkdir(basepath+indexPath, 0755)
}
