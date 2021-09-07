package rpc

import (
	"context"

	ctypes "github.com/celestiaorg/celestia-core/rpc/core/types"
	"github.com/celestiaorg/celestia-core/types"

	"github.com/celestiaorg/celestia-node/core"
)

const newBlockSubscriber = "NewBlock/Events"

// Client represents an RPC client designed to communicate
// with Celestia Core.
type Client struct {
	core.Client
}

// NewClient creates a new http.HTTP client that dials the given remote address,
// returning a wrapped http.HTTP client.
func NewClient(client core.Client) (*Client, error) {
	return &Client{
		client,
	}, nil
}

// GetStatus queries the remote address for its `Status`.
func (c *Client) GetStatus(ctx context.Context) (*ctypes.ResultStatus, error) {
	return c.Status(ctx)
}

// GetBlock queries the remote address for a `Block` at the given height.
func (c *Client) GetBlock(ctx context.Context, height *int64) (*ctypes.ResultBlock, error) {
	return c.Block(ctx, height)
}

// StartBlockSubscription subscribes to new block events from the remote address, returning
// an event channel on success. The Client must already be started in order to start the
// subscription.
func (c *Client) StartBlockSubscription(ctx context.Context) (<-chan ctypes.ResultEvent, error) {
	return c.Subscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlock).String())
}

// StopBlockSubscription stops the subscription to new block events from the remote address.
// TODO @renaynay: does it actually close the channel?
func (c *Client) StopBlockSubscription(ctx context.Context) error {
	return c.Unsubscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlockHeader).String())
}
