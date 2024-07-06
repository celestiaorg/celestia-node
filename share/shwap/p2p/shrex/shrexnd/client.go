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
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	pb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd/pb"
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
// Returns NamespacedData with unverified inclusion proofs against the share.Root.
func (c *Client) RequestND(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	peer peer.ID,
) (shwap.NamespacedData, error) {
	if err := namespace.ValidateForData(); err != nil {
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
	namespace share.Namespace,
	peerID peer.ID,
) (shwap.NamespacedData, error) {
	stream, err := c.host.NewStream(ctx, peerID, c.protocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	c.setStreamDeadlines(ctx, stream)

	req, err := shwap.NewNamespaceDataID(height, namespace)
	if err != nil {
		return nil, fmt.Errorf("client-nd: creating request: %w", err)
	}

	br, err := req.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("client-nd: marshaling request: %w", err)
	}

	_, err = stream.Write(br)
	if err != nil {
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSendReqErr)
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("client-nd: writing request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Debugw("client-nd: closing write side of the stream", "err", err)
	}

	if err := c.readStatus(ctx, stream); err != nil {
		return nil, err
	}
	return c.readNamespacedShares(ctx, stream)
}

func (c *Client) readStatus(ctx context.Context, stream network.Stream) error {
	var resp pb.GetSharesByNamespaceStatusResponse
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

// readNamespacedShares converts proto Rows to share.NamespacedData
func (c *Client) readNamespacedShares(
	ctx context.Context,
	stream network.Stream,
) (shwap.NamespacedData, error) {
	var shares shwap.NamespacedData
	for {
		var row pb.NamespaceRowResponse
		_, err := serde.Read(stream, &row)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// all data is received and steam is closed by server
				return shares, nil
			}
			c.metrics.ObserveRequests(ctx, 1, shrex.StatusReadRespErr)
			return nil, err
		}
		var proof nmt.Proof
		if row.Proof != nil {
			if len(row.Shares) != 0 {
				proof = nmt.NewInclusionProof(
					int(row.Proof.Start),
					int(row.Proof.End),
					row.Proof.Nodes,
					row.Proof.IsMaxNamespaceIgnored,
				)
			} else {
				proof = nmt.NewAbsenceProof(
					int(row.Proof.Start),
					int(row.Proof.End),
					row.Proof.Nodes,
					row.Proof.LeafHash,
					row.Proof.IsMaxNamespaceIgnored,
				)
			}
		}
		shares = append(shares, shwap.RowNamespaceData{
			Shares: row.Shares,
			Proof:  &proof,
		})
	}
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

func (c *Client) convertStatusToErr(ctx context.Context, status pb.StatusCode) error {
	switch status {
	case pb.StatusCode_OK:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
		return nil
	case pb.StatusCode_NOT_FOUND:
		c.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
		return shrex.ErrNotFound
	case pb.StatusCode_INVALID:
		log.Warn("client-nd: invalid request")
		fallthrough
	case pb.StatusCode_INTERNAL:
		fallthrough
	default:
		return shrex.ErrInvalidResponse
	}
}
