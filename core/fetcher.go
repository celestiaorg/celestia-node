package core

import (
	"context"
	"fmt"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/cometbft/cometbft/types"
	"github.com/gogo/protobuf/proto"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	libhead "github.com/celestiaorg/go-header"
)

const newBlockSubscriber = "NewBlock/Events"

type SignedBlock struct {
	Header       *types.Header       `json:"header"`
	Commit       *types.Commit       `json:"commit"`
	Data         *types.Data         `json:"data"`
	ValidatorSet *types.ValidatorSet `json:"validator_set"`
}

// BlockEvent is a new-block notification carrying the height plus which source
// announced it. The block body is fetched on demand via GetSignedBlockFrom,
// which prefers the announcing source — the peer fastest to notify this height,
// a fresh signal that it's the most responsive peer.
type BlockEvent struct {
	Height int64
	// source is the index of the MultiSource source that delivered this height
	// first. It is internal routing: zero and ignored for a single BlockFetcher.
	source int
	// addr is the announcing source's endpoint address, carried so the Listener
	// can name the offending source if its block is rejected (wrong chain or
	// inconsistent data) — making "remove the bad source" actionable.
	addr string
}

var (
	log                     = logging.Logger("core")
	newDataSignedBlockQuery = types.QueryForEvent(types.EventSignedBlock).String()
)

type BlockFetcher struct {
	client coregrpc.BlockAPIClient
	addr   string
}

// NewBlockFetcher returns a new `BlockFetcher`.
func NewBlockFetcher(conn *grpc.ClientConn) *BlockFetcher {
	return &BlockFetcher{
		client: coregrpc.NewBlockAPIClient(conn),
		addr:   conn.Target(),
	}
}

// GetBlockInfo queries Core for additional block information, like Commit and ValidatorSet.
func (f *BlockFetcher) GetBlockInfo(ctx context.Context, height int64) (*types.Commit, *types.ValidatorSet, error) {
	commit, err := f.Commit(ctx, height)
	if err != nil {
		return nil, nil, fmt.Errorf("core/fetcher: getting commit at height %d: %w", height, err)
	}

	// If a nil `height` is given as a parameter, there is a chance
	// that a new block could be produced between getting the latest
	// commit and getting the latest validator set. Therefore, it is
	// best to get the validator set at the latest commit's height to
	// prevent this potential inconsistency.
	valSet, err := f.ValidatorSet(ctx, commit.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("core/fetcher: getting validator set at height %d: %w", height, err)
	}

	return commit, valSet, nil
}

// GetBlock queries Core for a `Block` at the given height.
// if the height is nil, use the latest height
func (f *BlockFetcher) GetBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	stream, err := f.client.BlockByHeight(ctx, &coregrpc.BlockByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}
	block, err := f.receiveBlockByHeight(ctx, stream)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (f *BlockFetcher) GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error) {
	if hash == nil {
		return nil, fmt.Errorf("cannot get block with nil hash")
	}
	stream, err := f.client.BlockByHash(ctx, &coregrpc.BlockByHashRequest{Hash: hash})
	if err != nil {
		return nil, err
	}
	block, err := receiveBlockByHash(stream)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// GetSignedBlock queries Core for a `Block` at the given height.
// if the height is nil, use the latest height.
func (f *BlockFetcher) GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	stream, err := f.client.BlockByHeight(ctx, &coregrpc.BlockByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}
	return f.receiveBlockByHeight(ctx, stream)
}

// GetSignedBlockFrom fetches the block for the given event. A single
// BlockFetcher has only one source, so the event's source hint is irrelevant
// and it simply fetches by height.
func (f *BlockFetcher) GetSignedBlockFrom(ctx context.Context, ev BlockEvent) (*SignedBlock, error) {
	return f.GetSignedBlock(ctx, ev.Height)
}

// Commit queries Core for a `Commit` from the block at
// the given height.
// If the height is nil, use the latest height.
func (f *BlockFetcher) Commit(ctx context.Context, height int64) (*types.Commit, error) {
	res, err := f.client.Commit(ctx, &coregrpc.CommitRequest{Height: height})
	if err != nil {
		return nil, err
	}

	if res != nil && res.Commit == nil {
		return nil, fmt.Errorf("core/fetcher: commit not found at height %d", height)
	}

	commit, err := types.CommitFromProto(res.Commit)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

// ValidatorSet queries Core for the ValidatorSet from the
// block at the given height.
// If the height is nil, use the latest height.
func (f *BlockFetcher) ValidatorSet(ctx context.Context, height int64) (*types.ValidatorSet, error) {
	res, err := f.client.ValidatorSet(ctx, &coregrpc.ValidatorSetRequest{Height: height})
	if err != nil {
		return nil, err
	}

	if res != nil && res.ValidatorSet == nil {
		return nil, fmt.Errorf("core/fetcher: validator set not found at height %d", height)
	}

	validatorSet, err := types.ValidatorSetFromProto(res.ValidatorSet)
	if err != nil {
		return nil, err
	}

	return validatorSet, nil
}

// SubscribeNewBlockEvent subscribes to new block heights from Core, returning a
// channel of BlockEvents. The full block is intentionally NOT fetched here: the
// consumer (Listener) pulls the block on demand via GetSignedBlockFrom, so a
// MultiSource fanning N endpoints downloads each block once rather than once
// per source.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (chan BlockEvent, error) {
	eventCh := make(chan BlockEvent, 1)

	go func() {
		defer close(eventCh)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			subscription, err := f.client.SubscribeNewHeights(ctx, &coregrpc.SubscribeNewHeightsRequest{})
			if err != nil {
				log.Warnw("fetcher: failed to subscribe to new block events", "err", err)
				continue
			}

			log.Debug("fetcher: subscription created")
			err = f.receive(ctx, eventCh, subscription)
			if err != nil {
				log.Warnw("fetcher: error receiving new height", "err", err.Error())
				_ = subscription.CloseSend() // inform client that the subscription won't be used anymore
				continue
			}
		}
	}()
	return eventCh, nil
}

func (f *BlockFetcher) receive(
	ctx context.Context,
	eventCh chan BlockEvent,
	subscription coregrpc.BlockAPI_SubscribeNewHeightsClient,
) error {
	log.Debug("fetcher: started listening for new block heights")
	for {
		resp, err := subscription.Recv()
		if err != nil {
			return err
		}

		select {
		case eventCh <- BlockEvent{Height: resp.Height, addr: f.addr}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// IsSyncing returns the sync status of the Core connection: true for
// syncing, and false for already caught up. It can also return an error
// in the case of a failed status request.
func (f *BlockFetcher) IsSyncing(ctx context.Context) (bool, error) {
	resp, err := f.client.Status(ctx, &coregrpc.StatusRequest{})
	if err != nil {
		return false, err
	}
	return resp.SyncInfo.CatchingUp, nil
}

// IsSyncingFrom reports the sync status for the source that announced the
// event. A single BlockFetcher has only one source, so the event's source hint
// is irrelevant and it simply reports its own status.
func (f *BlockFetcher) IsSyncingFrom(ctx context.Context, _ BlockEvent) (bool, error) {
	return f.IsSyncing(ctx)
}

// Verify checks the core endpoint is on the expected network. The expected
// chain ID must be set: a node must know which network it serves.
func (f *BlockFetcher) Verify(ctx context.Context, expected string) error {
	if expected == "" {
		return fmt.Errorf("core/fetcher: expected chain ID must be configured")
	}
	id, err := f.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("core/fetcher: fetching chain ID: %w", err)
	}
	if id != expected {
		return fmt.Errorf("core/fetcher: network mismatch: expected %q, got %q", expected, id)
	}
	return nil
}

// ChainID returns the chain ID (network name) that the Core node is connected to.
// This can be used to validate that the node is connected to the correct network.
func (f *BlockFetcher) ChainID(ctx context.Context) (string, error) {
	resp, err := f.client.Status(ctx, &coregrpc.StatusRequest{})
	if err != nil {
		return "", err
	}
	if resp.NodeInfo == nil {
		return "", fmt.Errorf("core/fetcher: node info not available in status response")
	}
	return resp.NodeInfo.Network, nil
}

func (f *BlockFetcher) receiveBlockByHeight(ctx context.Context, streamer coregrpc.BlockAPI_BlockByHeightClient) (
	*SignedBlock,
	error,
) {
	span := trace.SpanFromContext(ctx)

	parts := make([]*tmproto.Part, 0)

	// receive the first part to get the block meta, commit, and validator set
	firstPart, err := streamer.Recv()
	if err != nil {
		return nil, err
	}
	span.AddEvent("fetcher: received first block part")

	commit, err := types.CommitFromProto(firstPart.Commit)
	if err != nil {
		return nil, err
	}
	validatorSet, err := types.ValidatorSetFromProto(firstPart.ValidatorSet)
	if err != nil {
		return nil, err
	}
	parts = append(parts, firstPart.BlockPart)

	// receive the rest of the block
	isLast := firstPart.IsLast
	for !isLast {
		resp, err := streamer.Recv()
		if err != nil {
			return nil, err
		}
		span.AddEvent("fetcher: received block part")

		parts = append(parts, resp.BlockPart)
		isLast = resp.IsLast
	}

	block, err := partsToBlock(parts)
	if err != nil {
		return nil, err
	}

	return &SignedBlock{
		Header:       &block.Header,
		Commit:       commit,
		Data:         &block.Data,
		ValidatorSet: validatorSet,
	}, nil
}

func receiveBlockByHash(streamer coregrpc.BlockAPI_BlockByHashClient) (*types.Block, error) {
	parts := make([]*tmproto.Part, 0)
	isLast := false
	for !isLast {
		resp, err := streamer.Recv()
		if err != nil {
			return nil, err
		}
		parts = append(parts, resp.BlockPart)
		isLast = resp.IsLast
	}
	return partsToBlock(parts)
}

// partsToBlock takes a slice of parts and generates the corresponding block.
// It empties the slice to optimize the memory usage.
func partsToBlock(parts []*tmproto.Part) (*types.Block, error) {
	partSet := types.NewPartSetFromHeader(types.PartSetHeader{
		Total: uint32(len(parts)),
	}, types.BlockPartSizeBytes)
	for _, part := range parts {
		ok, err := partSet.AddPartWithoutProof(&types.Part{Index: part.Index, Bytes: part.Bytes})
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("core/fetcher: failed to add part (index %d): duplicate or invalid", part.Index)
		}
	}
	pbb := new(tmproto.Block)
	bz := partSet.GetBytes()
	err := proto.Unmarshal(bz, pbb)
	if err != nil {
		return nil, err
	}
	block, err := types.BlockFromProto(pbb)
	if err != nil {
		return nil, err
	}
	return block, nil
}
