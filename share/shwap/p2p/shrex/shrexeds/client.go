package shrexeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
)

// Client is responsible for requesting EDSs for blocksync over the ShrEx/EDS protocol.
type Client struct {
	params     *Parameters
	protocolID protocol.ID
	host       host.Host

	metrics *shrex.Metrics
}

// NewClient creates a new ShrEx/EDS client.
func NewClient(params *Parameters, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: client creation failed: %w", err)
	}

	return &Client{
		params:     params,
		host:       host,
		protocolID: shrex.ProtocolID(params.NetworkID(), protocolString),
	}, nil
}

// RequestEDS requests the ODS from the given peers and returns the EDS upon success.
func (c *Client) RequestEDS(
	ctx context.Context,
	root *share.AxisRoots,
	height uint64,
	peer peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	eds, err := c.doRequest(ctx, root, height, peer)
	if err == nil {
		return eds, nil
	}
	log.Debugw("client: eds request to peer failed",
		"height", height,
		"peer", peer.String(),
		"error", err)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusTimeout)
		return nil, err
	}
	// some net.Errors also mean the context deadline was exceeded, but yamux/mocknet do not
	// unwrap to a ctx err
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusTimeout)
			return nil, context.DeadlineExceeded
		}
	}
	if !errors.Is(err, shrex.ErrNotFound) {
		log.Warnw("client: eds request to peer failed",
			"peer", peer.String(),
			"height", height,
			"err", err)
	}

	return nil, err
}

func (c *Client) doRequest(
	ctx context.Context,
	root *share.AxisRoots,
	height uint64,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	streamOpenCtx, cancel := context.WithTimeout(ctx, c.params.ServerReadTimeout)
	defer cancel()
	stream, err := c.host.NewStream(streamOpenCtx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer utils.CloseAndLog(log, "client", stream)

	c.setStreamDeadlines(ctx, stream)
	// request ODS
	log.Debugw("client: requesting ods",
		"height", height,
		"peer", to.String())
	id, err := shwap.NewEdsID(height)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	_, err = id.WriteTo(stream)
	if err != nil {
		return nil, fmt.Errorf("write request to stream: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Warnw("client: error closing write", "err", err)
	}

	// read and parse status from peer
	resp := new(shrexpb.Response)
	err = stream.SetReadDeadline(time.Now().Add(c.params.ServerReadTimeout))
	if err != nil {
		log.Debugw("client: failed to set read deadline for reading status", "err", err)
	}
	_, err = serde.Read(stream, resp)
	if err != nil {
		// server closes the stream here if we are rate limited
		if errors.Is(err, io.EOF) {
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusRateLimited)
			return nil, shrex.ErrNotFound
		}
		return nil, fmt.Errorf("read status from stream: %w", err)
	}
	switch resp.Status {
	case shrexpb.Status_OK:
		// reset stream deadlines to original values, since read deadline was changed during status read
		c.setStreamDeadlines(ctx, stream)
		// use header and ODS bytes to construct EDS and verify it against dataHash
		eds, err := eds.ReadEDS(ctx, stream, root)
		if err != nil {
			return nil, fmt.Errorf("read eds from stream: %w", err)
		}
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
		return eds, nil
	case shrexpb.Status_NOT_FOUND:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
		return nil, shrex.ErrNotFound
	case shrexpb.Status_INVALID:
		log.Debug("client: invalid request")
		fallthrough
	case shrexpb.Status_INTERNAL:
		fallthrough
	default:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusInternalErr)
		return nil, shrex.ErrInvalidResponse
	}
}

func (c *Client) setStreamDeadlines(ctx context.Context, stream network.Stream) {
	// set read/write deadline to use context deadline if it exists
	if dl, ok := ctx.Deadline(); ok {
		err := stream.SetDeadline(dl)
		if err == nil {
			return
		}
		log.Debugw("client: setting deadline: %s", "err", err)
	}

	// if deadline not set, client read deadline defaults to server write deadline
	if c.params.ServerWriteTimeout != 0 {
		err := stream.SetReadDeadline(time.Now().Add(c.params.ServerWriteTimeout))
		if err != nil {
			log.Debugw("client: setting read deadline", "err", err)
		}
	}

	// if deadline not set, client write deadline defaults to server read deadline
	if c.params.ServerReadTimeout != 0 {
		err := stream.SetWriteDeadline(time.Now().Add(c.params.ServerReadTimeout))
		if err != nil {
			log.Debugw("client: setting write deadline", "err", err)
		}
	}
}
