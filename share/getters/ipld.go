package getters

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var _ share.Getter = (*IPLDGetter)(nil)

// IPLDGetter is a share.Getter that retrieves shares from the IPLD network. Result caching is
// handled by the provided blockservice. A blockservice session will be created for retrieval if the
// passed context is wrapped with WithSession.
type IPLDGetter struct {
	rtrv  *eds.Retriever
	bServ blockservice.BlockService
}

// NewIPLDGetter creates a new share.Getter that retrieves shares from the IPLD network.
func NewIPLDGetter(bServ blockservice.BlockService) *IPLDGetter {
	return &IPLDGetter{
		rtrv:  eds.NewRetriever(bServ),
		bServ: bServ,
	}
}

func (ig *IPLDGetter) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	root, leaf := ipld.Translate(dah, row, col)
	blockGetter := getGetter(ctx, ig.bServ)
	nd, err := share.GetShare(ctx, blockGetter, root, leaf, len(dah.RowsRoots))
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve share: %w", err)
	}

	return nd, nil
}

func (ig *IPLDGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	// rtrv.Retrieve calls shares.GetShares until enough shares are retrieved to reconstruct the EDS
	eds, err := ig.rtrv.Retrieve(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve eds: %w", err)
	}
	return eds, nil
}

func (ig *IPLDGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
) (share.NamespacedShares, error) {
	err := verifyNIDSize(nID)
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: invalid namespace ID: %w", err)
	}

	blockGetter := getGetter(ctx, ig.bServ)
	return collectSharesByNamespace(ctx, blockGetter, root, nID)
}

var sessionKey = &session{}

// session is a struct that can optionally be passed by context to the share.Getter methods using
// WithSession to indicate that a blockservice session should be created.
type session struct {
	sync.Mutex
	atomic.Pointer[blockservice.Session]
}

// WithSession stores an empty session in the context, indicating that a blockservice session should
// be created.
func WithSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionKey, &session{})
}

func getGetter(ctx context.Context, service blockservice.BlockService) blockservice.BlockGetter {
	s, ok := ctx.Value(sessionKey).(*session)
	if !ok {
		return service
	}

	val := s.Load()
	if val != nil {
		return val
	}

	s.Lock()
	defer s.Unlock()
	val = s.Load()
	if val == nil {
		val = blockservice.NewSession(ctx, service)
		s.Store(val)
	}
	return val
}
