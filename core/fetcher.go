package core

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	logging "github.com/ipfs/go-log/v2"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	"github.com/tendermint/tendermint/types"
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

var (
	log                     = logging.Logger("core")
	newDataSignedBlockQuery = types.QueryForEvent(types.EventSignedBlock).String()
)

// BlockFetcher fetches blocks from core using one or more gRPC clients
type BlockFetcher struct {
	// clients holds all the BlockAPIClients for each connection
	clients []coregrpc.BlockAPIClient
	// currentClient is the index of the currently active client
	currentClient int32
}

// NewMultiBlockFetcher returns a new `BlockFetcher` using multiple connections.
// It implements client-side load balancing using a simple round-robin mechanism.
// If only one connection is provided, it will still use the same round-robin logic
// but will only have one client to select from.
func NewMultiBlockFetcher(conns []*grpc.ClientConn) (*BlockFetcher, error) {
	if len(conns) == 0 {
		return nil, fmt.Errorf("at least one connection is required")
	}

	clients := make([]coregrpc.BlockAPIClient, len(conns))
	for i, conn := range conns {
		clients[i] = coregrpc.NewBlockAPIClient(conn)
	}

	return &BlockFetcher{
		clients:       clients,
		currentClient: 0,
	}, nil
}

// getCurrentClient returns the current client and rotates to the next client
// in a round-robin fashion. This provides simple load balancing across
// multiple Core gRPC endpoints.
func (f *BlockFetcher) getCurrentClient() coregrpc.BlockAPIClient {
	// Get the current client
	current := atomic.LoadInt32(&f.currentClient)

	// Calculate the next client in round-robin fashion
	next := (current + 1) % int32(len(f.clients))

	// Update the current client atomically to avoid race conditions
	atomic.StoreInt32(&f.currentClient, next)

	return f.clients[current]
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
	client := f.getCurrentClient()
	stream, err := client.BlockByHeight(ctx, &coregrpc.BlockByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}
	block, err := receiveBlockByHeight(stream)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (f *BlockFetcher) GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error) {
	if hash == nil {
		return nil, fmt.Errorf("cannot get block with nil hash")
	}
	client := f.getCurrentClient()
	stream, err := client.BlockByHash(ctx, &coregrpc.BlockByHashRequest{Hash: hash})
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
	client := f.getCurrentClient()
	stream, err := client.BlockByHeight(ctx, &coregrpc.BlockByHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}
	return receiveBlockByHeight(stream)
}

// Commit queries Core for a `Commit` from the block at
// the given height.
// If the height is nil, use the latest height.
func (f *BlockFetcher) Commit(ctx context.Context, height int64) (*types.Commit, error) {
	client := f.getCurrentClient()
	res, err := client.Commit(ctx, &coregrpc.CommitRequest{Height: height})
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
	client := f.getCurrentClient()
	res, err := client.ValidatorSet(ctx, &coregrpc.ValidatorSetRequest{Height: height})
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

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (chan types.EventDataSignedBlock, error) {
	signedBlockCh := make(chan types.EventDataSignedBlock, 1)

	go func() {
		defer close(signedBlockCh)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Get the current client for this subscription attempt
			client := f.getCurrentClient()
			subscription, err := client.SubscribeNewHeights(ctx, &coregrpc.SubscribeNewHeightsRequest{})
			if err != nil {
				// try re-subscribe in case of any errors that can come during subscription. gRPC
				// retry mechanism has a back off on retries, so we don't need timers anymore.
				log.Warnw("fetcher: failed to subscribe to new block events", "err", err)
				continue
			}

			log.Debug("fetcher: subscription created")
			err = f.receive(ctx, signedBlockCh, subscription)
			if err != nil {
				log.Warnw("fetcher: error receiving new height", "err", err.Error())
				continue
			}
		}
	}()
	return signedBlockCh, nil
}

func (f *BlockFetcher) receive(
	ctx context.Context,
	signedBlockCh chan types.EventDataSignedBlock,
	subscription coregrpc.BlockAPI_SubscribeNewHeightsClient,
) error {
	log.Debug("fetcher: started listening for new blocks")
	for {
		resp, err := subscription.Recv()
		if err != nil {
			return err
		}

		// TODO(@vgonkivs): make timeout configurable
		withTimeout, ctxCancel := context.WithTimeout(ctx, 10*time.Second)
		signedBlock, err := f.GetSignedBlock(withTimeout, resp.Height)
		ctxCancel()
		if err != nil {
			log.Warnw("fetcher: error receiving signed block", "height", resp.Height, "err", err.Error())
			continue
		}

		select {
		case signedBlockCh <- types.EventDataSignedBlock{
			Header:       *signedBlock.Header,
			Commit:       *signedBlock.Commit,
			ValidatorSet: *signedBlock.ValidatorSet,
			Data:         *signedBlock.Data,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// IsSyncing returns the sync status of the Core connection: true for
// syncing, and false for already caught up. It can also return an error
// in the case of a failed status request.
func (f *BlockFetcher) IsSyncing(ctx context.Context) (bool, error) {
	client := f.getCurrentClient()
	resp, err := client.Status(ctx, &coregrpc.StatusRequest{})
	if err != nil {
		return false, err
	}
	return resp.SyncInfo.CatchingUp, nil
}

func receiveBlockByHeight(streamer coregrpc.BlockAPI_BlockByHeightClient) (
	*SignedBlock,
	error,
) {
	parts := make([]*tmproto.Part, 0)

	// receive the first part to get the block meta, commit, and validator set
	firstPart, err := streamer.Recv()
	if err != nil {
		return nil, err
	}
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
	})
	for _, part := range parts {
		ok, err := partSet.AddPartWithoutProof(&types.Part{Index: part.Index, Bytes: part.Bytes})
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, err
		}
	}
	pbb := new(tmproto.Block)
	bz, err := io.ReadAll(partSet.GetReader())
	if err != nil {
		return nil, err
	}
	err = proto.Unmarshal(bz, pbb)
	if err != nil {
		return nil, err
	}
	block, err := types.BlockFromProto(pbb)
	if err != nil {
		return nil, err
	}
	return block, nil
}
