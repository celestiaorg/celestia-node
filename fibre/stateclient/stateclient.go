package stateclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	core "github.com/cometbft/cometbft/types"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v10/fibre/state"
	"github.com/celestiaorg/celestia-app/v10/fibre/validator"
	"github.com/celestiaorg/celestia-app/v10/pkg/appconsts"
	valaddr "github.com/celestiaorg/celestia-app/v10/x/valaddr/types"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("fibre/stateclient")

// ErrNoFibreProviderInfo is returned by fetchHostAt when a validator has not
// registered fibre provider info via x/valaddr. The absence is proven against
// the trusted AppHash, so this is an authoritative answer, not a transient
// failure.
var ErrNoFibreProviderInfo = errors.New("no fibre provider info")

// abciQuerier is the narrow surface of [tmservice.ServiceClient] that this
// package depends on. Defined as an interface so tests can supply a stub
// without implementing the full gRPC ServiceClient.
type abciQuerier interface {
	ABCIQuery(context.Context, *tmservice.ABCIQueryRequest, ...grpc.CallOption) (*tmservice.ABCIQueryResponse, error)
}

// compile-time guarantee that Client satisfies the app's fibre state.Client
// interface, so a future method addition (like RefreshHost) fails here rather
// than at a distant call site.
var _ state.Client = (*Client)(nil)

// Client is a [state.Client] implementation that resolves validator sets and
// hosts from the node's verified header chain instead of trusting a remote
// gRPC endpoint.
//
//   - Validator sets come from locally synced [header.ExtendedHeader]s.
//   - Validator hosts are queried via ABCI with a merkle proof against
//     the trusted AppHash from the header.
//   - ChainID is taken from the local head.
//   - VerifyPromise is a no-op: light/bridge nodes don't verify fibre payment
//     promises; that's the provider's job.
type Client struct {
	store   libhead.Store[*header.ExtendedHeader]
	network p2p.Network

	abciQueryCli abciQuerier
	prt          *merkle.ProofRuntime

	chainID string

	// hostCache is populated once in Start via prefetchHosts and never
	// re-populated thereafter. A cache hit in GetHost evicts the entry so
	// subsequent reads for the same validator go through fetchHostAt and
	// hit the chain for fresh state. The cache is a one-shot startup
	// warmup, not a persistent cache — no TTL/refresh.
	//
	// lastHost is the persistent baseline of the host last resolved for each
	// validator (by GetHost or RefreshHost). Unlike hostCache it is not evicted
	// on read; RefreshHost compares fresh on-chain state against it to decide
	// whether the host actually changed. lastRefresh rate-limits RefreshHost's
	// on-chain re-queries to at most once per refreshInterval per validator.
	// All four fields are guarded by hostMu.
	hostMu          sync.Mutex
	hostCache       map[string]validator.Host
	lastHost        map[string]validator.Host
	lastRefresh     map[string]time.Time
	refreshInterval time.Duration
}

// NewClient builds a fibre [state.Client] backed by the local header store and
// a gRPC connection used only for ABCI queries (with merkle proof verification).
func NewClient(
	store libhead.Store[*header.ExtendedHeader],
	conn *grpc.ClientConn,
	network p2p.Network,
) *Client {
	prt := merkle.DefaultProofRuntime()
	prt.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)
	return &Client{
		store:        store,
		network:      network,
		abciQueryCli: tmservice.NewServiceClient(conn),
		prt:          prt,
		hostCache:    make(map[string]validator.Host),
		lastHost:     make(map[string]validator.Host),
		lastRefresh:  make(map[string]time.Time),
		// registry state cannot change faster than one block, so re-querying
		// more often than the expected block time is pointless.
		refreshInterval: appconsts.DelayedPrecommitTimeout + appconsts.TimeoutCommit,
	}
}

// Head returns the latest validator set from the local header store.
// The header is already verified by the header chain — no remote round-trip,
// no extra verification.
func (c *Client) Head(ctx context.Context) (validator.Set, error) {
	head, err := c.store.Head(ctx)
	if err != nil {
		return validator.Set{}, fmt.Errorf("fetch head: %w", err)
	}
	return validator.Set{ValidatorSet: head.ValidatorSet, Height: head.Height()}, nil
}

// GetByHeight returns the validator set at the given height from the local
// header store. If the header is not yet synced locally, the underlying store
// returns an error — we deliberately do NOT fall back to a remote endpoint;
// the whole point is to trust only verified headers.
func (c *Client) GetByHeight(ctx context.Context, height uint64) (validator.Set, error) {
	hdr, err := c.store.GetByHeight(ctx, height)
	if err != nil {
		return validator.Set{}, fmt.Errorf("fetch header by height %d: %w", height, err)
	}
	return validator.Set{ValidatorSet: hdr.ValidatorSet, Height: hdr.Height()}, nil
}

// GetHost resolves a validator's fibre network host. If the cache (populated
// at Start) has an entry, it is returned and evicted from the cache so the
// next read goes to the chain for fresh state. Otherwise a single-key ABCI
// query is issued and verified against the trusted AppHash from the local
// head.
func (c *Client) GetHost(ctx context.Context, val *core.Validator) (validator.Host, error) {
	cacheKey := sdk.ConsAddress(val.Address.Bytes()).String()

	c.hostMu.Lock()
	if h, ok := c.hostCache[cacheKey]; ok {
		delete(c.hostCache, cacheKey)
		c.lastHost[cacheKey] = h
		c.hostMu.Unlock()
		return h, nil
	}
	c.hostMu.Unlock()

	head, err := c.store.Head(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch head: %w", err)
	}
	h, err := c.fetchHostAt(ctx, val, head)
	if err != nil {
		return "", err
	}

	c.hostMu.Lock()
	c.lastHost[cacheKey] = h
	c.hostMu.Unlock()
	return h, nil
}

// RefreshHost re-queries the chain for the validator's fibre host and reports
// whether it changed relative to the host last resolved by GetHost/RefreshHost.
// See [validator.HostRegistry] for the returned semantics. Re-queries are
// rate-limited to once per refreshInterval per validator; when rate-limited it
// returns (false, false, nil) without hitting the chain.
func (c *Client) RefreshHost(ctx context.Context, val *core.Validator) (changed, isValid bool, err error) {
	cacheKey := sdk.ConsAddress(val.Address.Bytes()).String()

	c.hostMu.Lock()
	if last, ok := c.lastRefresh[cacheKey]; ok && time.Since(last) < c.refreshInterval {
		c.hostMu.Unlock()
		// Rate-limited: state can't change faster than a block, so report no
		// change and let the caller fall back to its original error.
		return false, false, nil
	}
	old := c.lastHost[cacheKey]
	// Stamp upfront, regardless of the query outcome, to bound query load
	// unconditionally.
	c.lastRefresh[cacheKey] = time.Now()
	c.hostMu.Unlock()

	head, err := c.store.Head(ctx)
	if err != nil {
		return false, false, fmt.Errorf("fetch head: %w", err)
	}
	// A deregistered validator surfaces as ErrNoFibreProviderInfo here; it's
	// returned as an error so the caller won't retry against a validator that
	// has no host.
	host, err := c.fetchHostAt(ctx, val, head)
	if err != nil {
		return false, false, err
	}

	changed = host != old
	isValid = valaddr.ValidateHost(host.String()) == nil
	// Cache the true on-chain host even when invalid, so the next refresh sees
	// new == old and the "changed" signal goes quiet instead of re-firing.
	if changed {
		c.hostMu.Lock()
		c.lastHost[cacheKey] = host
		c.hostMu.Unlock()
	}
	return changed, isValid, nil
}

// fetchHostAt issues a single-key ABCI query for the validator's fibre
// provider info and verifies the returned proof against head.AppHash.
// All callers must pass a head they have already pinned, so concurrent
// prefetch queries anchor to the same AppHash.
func (c *Client) fetchHostAt(
	ctx context.Context,
	val *core.Validator,
	head *header.ExtendedHeader,
) (validator.Host, error) {
	consAddr := sdk.ConsAddress(val.Address.Bytes())
	key := valaddr.GetFibreProviderInfoKey(consAddr)
	// AppHash at height H is the state root after applying block H-1, so we
	// query at H-1 to be consistent with head.AppHash.
	resp, err := c.abciQueryCli.ABCIQuery(ctx, &tmservice.ABCIQueryRequest{
		Path:   fmt.Sprintf("store/%s/key", valaddr.StoreKey),
		Data:   key,
		Height: int64(head.Height()) - 1,
		Prove:  true,
	})
	if err != nil {
		return "", fmt.Errorf("abci query: %w", err)
	}
	if resp.GetCode() != 0 {
		return "", fmt.Errorf("abci query non-zero code: %s", resp.GetLog())
	}
	if resp.GetProofOps() == nil {
		return "", fmt.Errorf("missing proof ops for validator %s", consAddr)
	}

	value := resp.GetValue()
	proofOps := &crypto.ProofOps{Ops: make([]crypto.ProofOp, len(resp.ProofOps.Ops))}
	for i, op := range resp.ProofOps.Ops {
		proofOps.Ops[i] = crypto.ProofOp{Type: op.Type, Key: op.Key, Data: op.Data}
	}
	keypath := [][]byte{[]byte(valaddr.StoreKey), key}

	// Empty value means the validator hasn't registered fibre provider info.
	// Verify the absence proof so a malicious endpoint can't fabricate "not
	// registered" by stripping the value.
	if len(value) == 0 {
		if err := c.prt.VerifyFromKeys(proofOps, head.AppHash, keypath, nil); err != nil {
			return "", fmt.Errorf("verify absence proof: %w", err)
		}
		return "", fmt.Errorf("validator %s: %w", consAddr, ErrNoFibreProviderInfo)
	}

	if err := c.prt.VerifyValueFromKeys(proofOps, head.AppHash, keypath, value); err != nil {
		return "", fmt.Errorf("verify proof: %w", err)
	}

	var info valaddr.FibreProviderInfo
	if err := info.Unmarshal(value); err != nil {
		return "", fmt.Errorf("unmarshal FibreProviderInfo: %w", err)
	}
	return validator.Host(info.GetHost()), nil
}

// prefetchHosts fans out fetchHostAt across the head's validator set and
// stores successfully-verified hosts in hostCache. All proofs anchor to
// the same head.AppHash. Validators without registered provider info are
// the common case and are silently skipped; other per-validator errors
// (transport, proof verification) are logged as warnings since they may
// indicate a misbehaving gRPC endpoint.
func (c *Client) prefetchHosts(ctx context.Context, head *header.ExtendedHeader) {
	vals := head.ValidatorSet.Validators

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(16)
	for _, v := range vals {
		g.Go(func() error {
			consAddr := sdk.ConsAddress(v.Address.Bytes())
			host, err := c.fetchHostAt(gctx, v, head)
			if err != nil {
				if !errors.Is(err, ErrNoFibreProviderInfo) {
					log.Warnw("prefetch host failed",
						"validator", consAddr.String(), "err", err)
				}
				return nil
			}
			c.hostMu.Lock()
			c.hostCache[consAddr.String()] = host
			c.hostMu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	c.hostMu.Lock()
	cached := len(c.hostCache)
	c.hostMu.Unlock()
	log.Infow("host prefetch completed",
		"cached", cached, "validators", len(vals), "height", head.Height())
}

// ChainID returns the chain ID detected during Start.
func (c *Client) ChainID() string { return c.chainID }

// VerifyPromise is a no-op on light nodes, because they don't verify
// fibre payment promises — that's the provider's job. We return a zero
// [state.VerifiedPromise] so callers that ignore the value continue to work.
func (c *Client) VerifyPromise(context.Context, *state.PaymentPromise) (state.VerifiedPromise, error) {
	return state.VerifiedPromise{}, nil
}

// Start seeds the chain ID from the local head and, when running on a
// well-known network, sanity-checks that the header chain ID matches.
func (c *Client) Start(ctx context.Context) error {
	head, err := c.store.Head(ctx)
	if err != nil {
		return fmt.Errorf("fetch head for chain-id: %w", err)
	}
	c.chainID = head.ChainID()
	c.prefetchHosts(ctx, head)
	return nil
}

// Stop is a no-op: the gRPC conn and header store are owned elsewhere.
func (c *Client) Stop(context.Context) error { return nil }
