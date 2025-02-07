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
	"google.golang.org/grpc/status"

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

type BlockFetcher struct {
	client coregrpc.BlockAPIClient

	doneCh               chan struct{}
	cancel               context.CancelFunc
	isListeningForBlocks atomic.Bool
}

// NewBlockFetcher returns a new `BlockFetcher`.
func NewBlockFetcher(conn *grpc.ClientConn) (*BlockFetcher, error) {
	return &BlockFetcher{
		client: coregrpc.NewBlockAPIClient(conn),
	}, nil
}

// Stop stops the block fetcher.
// The underlying gRPC connection needs to be stopped separately.
func (f *BlockFetcher) Stop(ctx context.Context) error {
	f.cancel()
	select {
	case <-f.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("fetcher: unsubscribe from new block events: %w", ctx.Err())
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
	return receiveBlockByHeight(stream)
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

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel on success.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan types.EventDataSignedBlock, error) {
	if f.isListeningForBlocks.Load() {
		return nil, fmt.Errorf("already subscribed to new blocks")
	}
	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.doneCh = make(chan struct{})
	f.isListeningForBlocks.Store(true)

	subscription, err := f.client.SubscribeNewHeights(ctx, &coregrpc.SubscribeNewHeightsRequest{})
	if err != nil {
		close(f.doneCh)
		f.isListeningForBlocks.Store(false)
		return nil, err
	}

	log.Debug("created a subscription. Start listening for new blocks...")
	signedBlockCh := make(chan types.EventDataSignedBlock)
	go func() {
		defer close(f.doneCh)
		defer close(signedBlockCh)
		defer func() { f.isListeningForBlocks.Store(false) }()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := subscription.Recv()
				if err != nil {
					log.Errorw("fetcher: error receiving new height", "err", err.Error())
					_, ok := status.FromError(err) // parsing the gRPC error
					if ok {
						// ok means that err contains a gRPC status error.
						// move on another round of resubscribing.
						return
					}
					continue
				}
				withTimeout, ctxCancel := context.WithTimeout(ctx, 10*time.Second)
				signedBlock, err := f.GetSignedBlock(withTimeout, resp.Height)
				ctxCancel()
				if err != nil {
					log.Errorw("fetcher: error receiving signed block", "height", resp.Height, "err", err.Error())
					// sleeping a bit to avoid retrying instantly and give time for the gRPC connection
					// to recover automatically.
					time.Sleep(time.Second)
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
					return
				}
			}
		}
	}()

	return signedBlockCh, nil
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
