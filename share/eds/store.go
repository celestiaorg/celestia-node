package eds

import (
	"os"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/ipfs/go-datastore"
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
	err := r.Register("fs", &mount.FSMount{FS: os.DirFS(basepath + "/blocks/")})

	if err != nil {
		return nil, err
	}

	fsRepo, err := index.NewFSRepo(basepath + "/index/")
	if err != nil {
		return nil, err
	}

	invertedRepo := index.NewInverted(ds)

	dagStore, err := dagstore.NewDAGStore(
		dagstore.Config{
			TransientsDir: basepath + "/transients/",
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
