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

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexeds/pb"
)

// Client is responsible for requesting EDSs for blocksync over the ShrEx/EDS protocol.
type Client struct {
	params     *Parameters
	protocolID protocol.ID
	host       host.Host

	metrics *p2p.Metrics
}

// NewClient creates a new ShrEx/EDS client.
func NewClient(params *Parameters, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: client creation failed: %w", err)
	}

	return &Client{
		params:     params,
		host:       host,
		protocolID: p2p.ProtocolID(params.NetworkID(), protocolString),
	}, nil
}

// RequestEDS requests the ODS from the given peers and returns the EDS upon success.
func (c *Client) RequestEDS(
	ctx context.Context,
	dataHash share.DataHash,
	peer peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	eds, err := c.doRequest(ctx, dataHash, peer)
	if err == nil {
		return eds, nil
	}
	log.Debugw("client: eds request to peer failed", "peer", peer.String(), "hash", dataHash.String(), "error", err)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		c.metrics.ObserveRequests(ctx, 1, p2p.StatusTimeout)
		return nil, err
	}
	// some net.Errors also mean the context deadline was exceeded, but yamux/mocknet do not
	// unwrap to a ctx err
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
			c.metrics.ObserveRequests(ctx, 1, p2p.StatusTimeout)
			return nil, context.DeadlineExceeded
		}
	}
	if !errors.Is(err, p2p.ErrNotFound) {
		log.Warnw("client: eds request to peer failed",
			"peer", peer.String(),
			"hash", dataHash.String(),
			"err", err)
	}

	return nil, err
}

func (c *Client) doRequest(
	ctx context.Context,
	dataHash share.DataHash,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	streamOpenCtx, cancel := context.WithTimeout(ctx, c.params.ServerReadTimeout)
	defer cancel()
	stream, err := c.host.NewStream(streamOpenCtx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	c.setStreamDeadlines(ctx, stream)

	req := &pb.EDSRequest{Hash: dataHash}

	// request ODS
	log.Debugw("client: requesting ods", "hash", dataHash.String(), "peer", to.String())
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to write request to stream: %w", err)
	}
	err = stream.CloseWrite()
	if err != nil {
		log.Debugw("client: error closing write", "err", err)
	}

	// read and parse status from peer
	resp := new(pb.EDSResponse)
	err = stream.SetReadDeadline(time.Now().Add(c.params.ServerReadTimeout))
	if err != nil {
		log.Debugw("client: failed to set read deadline for reading status", "err", err)
	}
	_, err = serde.Read(stream, resp)
	if err != nil {
		// server closes the stream here if we are rate limited
		if errors.Is(err, io.EOF) {
			c.metrics.ObserveRequests(ctx, 1, p2p.StatusRateLimited)
			return nil, p2p.ErrNotFound
		}
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to read status from stream: %w", err)
	}

	switch resp.Status {
	case pb.Status_OK:
		// reset stream deadlines to original values, since read deadline was changed during status read
		c.setStreamDeadlines(ctx, stream)
		// use header and ODS bytes to construct EDS and verify it against dataHash
		eds, err := eds.ReadEDS(ctx, stream, dataHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
		}
		c.metrics.ObserveRequests(ctx, 1, p2p.StatusSuccess)
		return eds, nil
	case pb.Status_NOT_FOUND:
		c.metrics.ObserveRequests(ctx, 1, p2p.StatusNotFound)
		return nil, p2p.ErrNotFound
	case pb.Status_INVALID:
		log.Debug("client: invalid request")
		fallthrough
	case pb.Status_INTERNAL:
		fallthrough
	default:
		c.metrics.ObserveRequests(ctx, 1, p2p.StatusInternalErr)
		return nil, p2p.ErrInvalidResponse
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
