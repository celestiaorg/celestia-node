package getters

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-blockservice"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var _ share.Getter = (*IPLDGetter)(nil)

// IPLDGetter is a share.Getter that retrieves shares from the bitswap network. Result caching is
// handled by the provided blockservice. A blockservice session will be created for retrieval if the
// passed context is wrapped with WithSession.
type IPLDGetter struct {
	rtrv  *eds.Retriever
	bServ blockservice.BlockService
}

// NewIPLDGetter creates a new share.Getter that retrieves shares from the bitswap network.
func NewIPLDGetter(bServ blockservice.BlockService) *IPLDGetter {
	return &IPLDGetter{
		rtrv:  eds.NewRetriever(bServ),
		bServ: bServ,
	}
}

// GetShare gets a single share at the given EDS coordinates from the bitswap network.
func (ig *IPLDGetter) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	var err error
	ctx, span := tracer.Start(ctx, "ipld/get-share", trace.WithAttributes(
		attribute.String("root", dah.String()),
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	root, leaf := ipld.Translate(dah, row, col)

	// wrap the blockservice in a session if it has been signaled in the context.
	blockGetter := getGetter(ctx, ig.bServ)
	s, err := ipld.GetShare(ctx, blockGetter, root, leaf, len(dah.RowRoots))
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve share: %w", err)
	}

	return s, nil
}

func (ig *IPLDGetter) GetEDS(ctx context.Context, root *share.Root) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "ipld/get-eds", trace.WithAttributes(
		attribute.String("root", root.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	// rtrv.Retrieve calls shares.GetShares until enough shares are retrieved to reconstruct the EDS
	eds, err = ig.rtrv.Retrieve(ctx, root)
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve eds: %w", err)
	}
	return eds, nil
}

func (ig *IPLDGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "ipld/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	if err = namespace.ValidateForData(); err != nil {
		return nil, err
	}

	// wrap the blockservice in a session if it has been signaled in the context.
	blockGetter := getGetter(ctx, ig.bServ)
	shares, err = collectSharesByNamespace(ctx, blockGetter, root, namespace)
	if errors.Is(err, ipld.ErrNodeNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/ipld: failed to retrieve shares by namespace: %w", err)
	}
	return shares, nil
}

var sessionKey = &session{}

// session is a struct that can optionally be passed by context to the share.Getter methods using
// WithSession to indicate that a blockservice session should be created.
type session struct {
	sync.Mutex
	atomic.Pointer[blockservice.Session]
	ctx context.Context
}

// WithSession stores an empty session in the context, indicating that a blockservice session should
// be created.
func WithSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionKey, &session{ctx: ctx})
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
		val = blockservice.NewSession(s.ctx, service)
		s.Store(val)
	}
	return val
}
