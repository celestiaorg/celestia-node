package stateclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	core "github.com/cometbft/cometbft/types"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v9/fibre/state"
	"github.com/celestiaorg/celestia-app/v9/fibre/validator"
	valaddr "github.com/celestiaorg/celestia-app/v9/x/valaddr/types"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// hostCacheTTL bounds how long a successful GetHost lookup is reused without
// re-querying ABCI. Validator host changes are operator-driven and rare, so a
// short TTL is enough to absorb burst lookups during upload without staleness.
const hostCacheTTL = time.Minute

// abciQuerier is the narrow surface of [tmservice.ServiceClient] that this
// package depends on. Defined as an interface so tests can supply a stub
// without implementing the full gRPC ServiceClient.
type abciQuerier interface {
	ABCIQuery(context.Context, *tmservice.ABCIQueryRequest, ...grpc.CallOption) (*tmservice.ABCIQueryResponse, error)
}

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

	hostMu    sync.Mutex
	hostCache map[string]hostCacheEntry
}

type hostCacheEntry struct {
	host    validator.Host
	expires time.Time
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
		hostCache:    make(map[string]hostCacheEntry),
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

// GetHost resolves a validator's fibre network host via ABCI with merkle proof
// verification against the trusted AppHash from the local head.
// Successful lookups are cached for [hostCacheTTL]; misses and errors are not
// cached so transient failures don't get pinned.
func (c *Client) GetHost(ctx context.Context, val *core.Validator) (validator.Host, error) {
	consAddr := sdk.ConsAddress(val.Address.Bytes())
	cacheKey := consAddr.String()
	if host, ok := c.lookupHostCache(cacheKey); ok {
		return host, nil
	}

	head, err := c.store.Head(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch head: %w", err)
	}

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
		return "", fmt.Errorf("no fibre provider info for validator %s", consAddr)
	}

	if err := c.prt.VerifyValueFromKeys(proofOps, head.AppHash, keypath, value); err != nil {
		return "", fmt.Errorf("verify proof: %w", err)
	}

	var info valaddr.FibreProviderInfo
	if err := info.Unmarshal(value); err != nil {
		return "", fmt.Errorf("unmarshal FibreProviderInfo: %w", err)
	}
	host := validator.Host(info.GetHost())
	c.storeHostCache(cacheKey, host)
	return host, nil
}

func (c *Client) lookupHostCache(key string) (validator.Host, bool) {
	c.hostMu.Lock()
	defer c.hostMu.Unlock()

	entry, ok := c.hostCache[key]
	if !ok {
		return "", false
	}

	if time.Now().After(entry.expires) {
		delete(c.hostCache, key)
		return "", false
	}
	return entry.host, true
}

func (c *Client) storeHostCache(key string, host validator.Host) {
	c.hostMu.Lock()
	c.hostCache[key] = hostCacheEntry{host: host, expires: time.Now().Add(hostCacheTTL)}
	c.hostMu.Unlock()
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

	if c.chainID != c.network.String() {
		return fmt.Errorf(
			"fibre state client: chain-id mismatch: header=%q network=%q",
			c.chainID, c.network,
		)
	}
	return nil
}

// Stop is a no-op: the gRPC conn and header store are owned elsewhere.
func (c *Client) Stop(context.Context) error { return nil }
