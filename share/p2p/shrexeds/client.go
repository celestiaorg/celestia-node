package shrexeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
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
	protocolID protocol.ID
	host       host.Host
}

// NewClient creates a new ShrEx/EDS client.
func NewClient(params *Parameters, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: client creation failed: %w", err)
	}

	return &Client{
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
	resultChan := make(chan *rsmt2d.ExtendedDataSquare, 1)
	errChan := make(chan error, 1)

	// this is a hotfix for something inside of doRequest not respecting context.
	// should be reverted when closing issue #2109
	go func() {
		eds, err := c.doRequest(ctx, dataHash, peer)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- eds
	}()

	select {
	case eds := <-resultChan:
		return eds, nil
	case err := <-errChan:
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, ctx.Err()
		}

		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			if deadline, _ := ctx.Deadline(); deadline.Before(time.Now()) {
				return nil, context.DeadlineExceeded
			}
		}

		if err != p2p.ErrNotFound {
			log.Warnw("client: eds request to peer failed",
				"peer", peer,
				"hash", dataHash.String(),
				"err", err)
		}

		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
			log.Debugw("client: error setting deadline: %s", "err", err)
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
		log.Debugw("client: error closing write", "err", err)
	}

	// read and parse status from peer
	resp := new(pb.EDSResponse)
	_, err = serde.Read(stream, resp)
	if err != nil {
		// server is overloaded and closed the stream
		if errors.Is(err, io.EOF) {
			return nil, p2p.ErrNotFound
		}
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
	case pb.Status_NOT_FOUND:
		return nil, p2p.ErrNotFound
	case pb.Status_INVALID:
		log.Debug("client: invalid request")
		fallthrough
	case pb.Status_INTERNAL:
		fallthrough
	default:
		return nil, p2p.ErrInvalidResponse
	}
}
