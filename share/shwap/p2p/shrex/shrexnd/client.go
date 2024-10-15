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
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
)

// Client implements client side of shrex/nd protocol to obtain namespaced shares data from remote
// peers.
type Client struct {
	params     *Parameters
	protocolID protocol.ID

	host    host.Host
	metrics *shrex.Metrics
}

// NewClient creates a new shrEx/nd client
func NewClient(params *Parameters, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: client creation failed: %w", err)
	}

	return &Client{
		host:       host,
		protocolID: shrex.ProtocolID(params.NetworkID(), protocolString),
		params:     params,
	}, nil
}

// RequestND requests namespaced data from the given peer.
// Returns NamespaceData with unverified inclusion proofs against the share.Root.
func (c *Client) RequestND(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	peer peer.ID,
) (shwap.NamespaceData, error) {
	if err := libshare.ValidateForData(namespace); err != nil {
		return nil, err
	}

	shares, err := c.doRequest(ctx, height, namespace, peer)
	if err == nil {
		return shares, nil
	}
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
	if !errors.Is(err, shrex.ErrNotFound) && errors.Is(err, shrex.ErrRateLimited) {
		log.Warnw("client-nd: peer returned err", "err", err)
	}
	return nil, err
}

func (c *Client) doRequest(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	peerID peer.ID,
) (shwap.NamespaceData, error) {
	streamOpenCtx, cancel := context.WithTimeout(ctx, c.params.ServerReadTimeout)
	defer cancel()
	stream, err := c.host.NewStream(streamOpenCtx, peerID, c.protocolID)
	if err != nil {
		return nil, err
	}
	defer utils.CloseAndLog(log, "client", stream)

	c.setStreamDeadlines(ctx, stream)

	req, err := shwap.NewNamespaceDataID(height, namespace)
	if err != nil {
		return nil, fmt.Errorf("client-nd: creating request: %w", err)
	}

	_, err = req.WriteTo(stream)
	if err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSendReqErr)
		return nil, fmt.Errorf("client-nd: writing request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Warnw("client-nd: closing write side of the stream", "err", err)
	}

	if err := c.readStatus(ctx, stream); err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		return nil, err
	}

	nd := shwap.NamespaceData{}
	_, err = nd.ReadFrom(stream)
	if err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		return nil, err
	}
	return nd, nil
}

func (c *Client) readStatus(ctx context.Context, stream network.Stream) error {
	var resp shrexpb.Response
	_, err := serde.Read(stream, &resp)
	if err != nil {
		// server is overloaded and closed the stream
		if errors.Is(err, io.EOF) {
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusRateLimited)
			return shrex.ErrRateLimited
		}
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
		stream.Reset() //nolint:errcheck
		return fmt.Errorf("client-nd: reading status response: %w", err)
	}

	return c.convertStatusToErr(ctx, resp.Status)
}

func (c *Client) setStreamDeadlines(ctx context.Context, stream network.Stream) {
	// set read/write deadline to use context deadline if it exists
	deadline, ok := ctx.Deadline()
	if ok {
		err := stream.SetDeadline(deadline)
		if err == nil {
			return
		}
		log.Debugw("client-nd: set stream deadline", "err", err)
	}

	// if deadline not set, client read deadline defaults to server write deadline
	if c.params.ServerWriteTimeout != 0 {
		err := stream.SetReadDeadline(time.Now().Add(c.params.ServerWriteTimeout))
		if err != nil {
			log.Debugw("client-nd: set read deadline", "err", err)
		}
	}

	// if deadline not set, client write deadline defaults to server read deadline
	if c.params.ServerReadTimeout != 0 {
		err := stream.SetWriteDeadline(time.Now().Add(c.params.ServerReadTimeout))
		if err != nil {
			log.Debugw("client-nd: set write deadline", "err", err)
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
	case shrexpb.Status_INVALID:
		log.Warn("client-nd: invalid request")
		fallthrough
	case shrexpb.Status_INTERNAL:
		fallthrough
	default:
		return shrex.ErrInvalidResponse
	}
}
