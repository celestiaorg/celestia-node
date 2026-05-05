package core

import (
	"context"

	"github.com/cometbft/cometbft/types"

	libhead "github.com/celestiaorg/go-header"
)

// Fetcher is the behavioural contract that the consensus-block consumers
// (Listener, Exchange) depend on. The concrete *BlockFetcher implements it
// against a single endpoint; a future MultiBlockFetcher will implement the
// same contract against several endpoints with health-aware routing.
//
// Keeping this surface small and stable is intentional: the multi-endpoint
// router is a drop-in substitute and must not require changes to its
// consumers.
type Fetcher interface {
	// SubscribeNewBlockEvent opens a stream of new finalized blocks from the
	// consensus node. The returned channel is closed when ctx is cancelled.
	SubscribeNewBlockEvent(ctx context.Context) (chan SignedBlock, error)

	// GetBlock fetches a full block at the given height.
	GetBlock(ctx context.Context, height int64) (*SignedBlock, error)

	// GetSignedBlock fetches a full signed block at the given height.
	GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error)

	// GetBlockByHash fetches a block by its hash.
	GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error)

	// GetBlockInfo returns the commit and validator set for the block at the
	// given height. Implementations must source-bind the pair: the commit and
	// validator set must come from the same endpoint (or one whose coverage
	// strictly includes that height) to avoid mixing inconsistent metadata.
	GetBlockInfo(ctx context.Context, height int64) (*types.Commit, *types.ValidatorSet, error)

	// Commit fetches the commit for the block at the given height.
	Commit(ctx context.Context, height int64) (*types.Commit, error)

	// ValidatorSet fetches the validator set at the given height.
	ValidatorSet(ctx context.Context, height int64) (*types.ValidatorSet, error)

	// Status returns a coalesced view of the consensus node's status.
	// For multi-endpoint implementations this is an aggregate view (e.g.
	// CatchingUp == false if any healthy endpoint is caught up).
	Status(ctx context.Context) (*Status, error)

	// IsSyncing reports whether the consensus node is still catching up.
	IsSyncing(ctx context.Context) (bool, error)

	// ChainID returns the chain ID (network name) of the consensus node.
	ChainID(ctx context.Context) (string, error)
}

// compile-time assertion: *BlockFetcher must satisfy Fetcher.
var _ Fetcher = (*BlockFetcher)(nil)
