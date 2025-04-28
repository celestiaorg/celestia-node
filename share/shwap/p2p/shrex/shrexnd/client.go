package shrexnd

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

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
)

// Client implements client side of shrex protocol to obtain data from remote
// peers.
type Client struct {
	params *Parameters

	host    host.Host
	metrics *shrex.Metrics
}

// NewClient creates a new shrEx/nd client
func NewClient(params *Parameters, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-client: creation failed: %w", err)
	}
	return &Client{
		host:   host,
		params: params,
	}, nil
}

func (c *Client) Get(
	ctx context.Context,
	id id,
	container container,
	peer peer.ID,
) error {
	err := c.doRequest(ctx, id, container, peer)
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusTimeout)
		return err
	}
	// some net.Errors also mean the context deadline was exceeded, but yamux/mocknet do not
	// unwrap to a ctx err
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusTimeout)
			return context.DeadlineExceeded
		}
	}
	if !errors.Is(err, shrex.ErrNotFound) && errors.Is(err, shrex.ErrRateLimited) {
		log.Warnw("shrex-client: peer returned err", "err", err)
	}
	return err
}

func (c *Client) doRequest(
	ctx context.Context,
	id id,
	container container,
	peerID peer.ID,
) error {
	streamOpenCtx, cancel := context.WithTimeout(ctx, c.params.ServerReadTimeout)
	defer cancel()

	stream, err := c.host.NewStream(streamOpenCtx, peerID, shrex.ProtocolID(c.params.NetworkID(), id.Name()))
	if err != nil {
		return err
	}
	defer utils.CloseAndLog(log, "client", stream)

	c.setStreamDeadlines(ctx, stream)

	_, err = id.WriteTo(stream)
	if err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSendReqErr)
		return fmt.Errorf("shrex-client: writing request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Warnw("shrex-client: closing write side of the stream", "err", err)
	}

	status, err := c.readStatus(ctx, stream)
	if err != nil {
		// metrics updated inside read status
		return fmt.Errorf("shrex-client: reading status response: %w", err)
	}

	err = c.convertStatusToErr(ctx, status)
	if err != nil {
		return err
	}

	_, err = container.ReadFrom(stream)
	if err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		return err
	}
	return nil
}

func (c *Client) readStatus(ctx context.Context, stream network.Stream) (shrexpb.Status, error) {
	var resp shrexpb.Response
	_, err := serde.Read(stream, &resp)
	if err != nil {
		// server is overloaded and closed the stream
		if errors.Is(err, io.EOF) {
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusRateLimited)
			return shrexpb.Status_INTERNAL, shrex.ErrRateLimited
		}
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		return shrexpb.Status_INTERNAL, err
	}
	return resp.Status, nil
}

func (c *Client) setStreamDeadlines(ctx context.Context, stream network.Stream) {
	// set read/write deadline to use context deadline if it exists
	deadline, ok := ctx.Deadline()
	if ok {
		err := stream.SetDeadline(deadline)
		if err == nil {
			return
		}
		log.Debugw("shrex-client: set stream deadline", "err", err)
	}

	// if deadline not set, client read deadline defaults to server write deadline
	if c.params.ServerWriteTimeout != 0 {
		err := stream.SetReadDeadline(time.Now().Add(c.params.ServerWriteTimeout))
		if err != nil {
			log.Debugw("shrex-client: set read deadline", "err", err)
		}
	}

	// if deadline not set, client write deadline defaults to server read deadline
	if c.params.ServerReadTimeout != 0 {
		err := stream.SetWriteDeadline(time.Now().Add(c.params.ServerReadTimeout))
		if err != nil {
			log.Debugw("shrex-client: set write deadline", "err", err)
		}
	}
}

func (c *Client) convertStatusToErr(ctx context.Context, status shrexpb.Status) error {
	switch status {
	case shrexpb.Status_OK:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
		return nil
	case shrexpb.Status_NOT_FOUND:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
		return shrex.ErrNotFound
	case shrexpb.Status_INTERNAL:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusInternalErr)
		return shrex.ErrInternalServer
	case shrexpb.Status_INVALID:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusBadRequest)
		return shrex.ErrInvalidRequest
	default:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		return shrex.ErrInvalidResponse
	}
}
