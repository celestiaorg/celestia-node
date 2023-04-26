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

const safetyTimeout = time.Millisecond * 500

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
	eds, err := c.doSafeRequest(ctx, dataHash, peer)
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
	if err != p2p.ErrNotFound {
		log.Warnw("client: eds request to peer failed",
			"peer", peer,
			"hash", dataHash.String(),
			"err", err)
	}

	return nil, err
}

type result struct {
	eds *rsmt2d.ExtendedDataSquare
	err error
}

func (c *Client) doSafeRequest(
	ctx context.Context,
	dataHash share.DataHash,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	resultChan := make(chan result)
	stream, err := c.host.NewStream(ctx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	// if there is a deadline set, safetyContext will give the request 500 milliseconds to recognize the context
	// has deadlined before canceling the operation itself.
	// this is a hotfix until we solve why doRequest does not respect context. Related: issue #2109
	safetyContext, cancel := context.WithCancel(ctx)
	if dl, ok := ctx.Deadline(); ok {
		safetyContext, cancel = context.WithDeadline(context.Background(), dl.Add(safetyTimeout))
		if err = stream.SetDeadline(dl); err != nil {
			log.Debugw("client: error setting deadline: %s", "err", err)
		}
	}
	defer cancel()

	go doRequest(ctx, stream, dataHash, to, resultChan)

	select {
	case res := <-resultChan:
		if errors.Is(res.err, context.DeadlineExceeded) || errors.Is(res.err, context.Canceled) {
			log.Debugw("client: doRequest successfully respected context deadline",
				"datahash", dataHash.String(),
				"peer", to,
			)
		}
		return res.eds, res.err
	case <-safetyContext.Done():
		stream.Reset() //nolint:errcheck
		log.Errorw("client: doRequest failed to respect context after safety timeout",
			"datahash", dataHash.String(),
			"peer", to,
		)
		return nil, safetyContext.Err()
	}
}

func doRequest(
	ctx context.Context,
	stream network.Stream,
	dataHash share.DataHash,
	to peer.ID,
	resultChan chan result,
) {
	req := &pb.EDSRequest{Hash: dataHash}

	// request ODS
	log.Debugf("client: requesting ods %s from peer %s", dataHash.String(), to)
	_, err := serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		resultChan <- result{nil, fmt.Errorf("failed to write request to stream: %w", err)}
		return
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
			resultChan <- result{nil, p2p.ErrNotFound}
			return
		}
		stream.Reset() //nolint:errcheck
		resultChan <- result{nil, fmt.Errorf("failed to read status from stream: %w", err)}
		return
	}

	var res result
	switch resp.Status {
	case pb.Status_OK:
		// use header and ODS bytes to construct EDS and verify it against dataHash
		res.eds, res.err = eds.ReadEDS(ctx, stream, dataHash)
		if res.err != nil {
			res.err = fmt.Errorf("failed to read eds from ods bytes: %w", res.err)
		}
	case pb.Status_NOT_FOUND:
		res.err = p2p.ErrNotFound
	case pb.Status_INVALID:
		log.Debug("client: invalid request")
		fallthrough
	case pb.Status_INTERNAL:
		fallthrough
	default:
		res.err = p2p.ErrInvalidResponse
	}

	resultChan <- res
}
