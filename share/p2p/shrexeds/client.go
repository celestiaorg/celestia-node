package shrexeds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexeds/pb"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/rsmt2d"
)

// Client is responsible for requesting EDSs for blocksync over the ShrEx/EDS protocol.
type Client struct {
	protocolID protocol.ID
	host       host.Host
}

// NewClient creates a new ShrEx/EDS client.
func NewClient(host host.Host, opts ...Option) (*Client, error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(params)
	}

	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: client creation failed: %w", err)
	}

	return &Client{
		host:       host,
		protocolID: protocolID(params.protocolSuffix),
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
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil, ctx.Err()
	}
	// some net.Errors also mean the context deadline was exceeded, but yamux/mocknet do not
	// unwrap to a ctx err
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
			return nil, context.DeadlineExceeded
		}
	}
	if err != p2p.ErrUnavailable {
		log.Errorw("client: eds request to peer failed", "peer", peer, "hash", dataHash.String())
	}
	return nil, err
}

func (c *Client) doRequest(
	ctx context.Context,
	dataHash share.DataHash,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	stream, err := c.host.NewStream(ctx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	if dl, ok := ctx.Deadline(); ok {
		if err = stream.SetDeadline(dl); err != nil {
			log.Debugw("error setting deadline: %s", err)
		}
	}

	req := &pb.EDSRequest{Hash: dataHash}

	// request ODS
	log.Debugf("client: requesting ods %s from peer %s", dataHash.String(), to)
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to write request to stream: %w", err)
	}
	err = stream.CloseWrite()
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to close write on stream: %w", err)
	}

	// read and parse status from peer
	resp := new(pb.EDSResponse)
	_, err = serde.Read(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to read status from stream: %w", err)
	}

	switch resp.Status {
	case pb.Status_OK:
		// use header and ODS bytes to construct EDS and verify it against dataHash
		eds, err := eds.ReadEDS(ctx, stream, dataHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
		}
		return eds, nil
	case pb.Status_NOT_FOUND, pb.Status_REFUSED:
		log.Debugf("client: peer %s couldn't serve eds %s with status %s", to.String(), dataHash.String(), resp.GetStatus())
		return nil, p2p.ErrUnavailable
	case pb.Status_INVALID:
		fallthrough
	default:
		log.Errorf("request status %s returned for root %s", resp.Status.String(), dataHash.String())
		return nil, p2p.ErrInvalidResponse
	}
}
