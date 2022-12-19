package eds

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

var _ share.Getter = (*Getter)(nil)

func NewGetter(s *Store) *Getter {
	return &Getter{store: s}
}

type Getter struct {
	store *Store
}

func (c *Getter) GetBlobByNamespace(ctx context.Context, root *share.Root, id namespace.ID) (*share.Blob, error) {
	panic("implement me")
}
