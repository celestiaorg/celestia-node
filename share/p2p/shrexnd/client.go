package shrexnd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/nmt/namespace"
)

// Client implements client side of shrex/nd protocol to obtain namespaced shares data from remote
// peers.
type Client struct {
	params     *Parameters
	protocolID protocol.ID

	host host.Host
}

// NewClient creates a new shrEx/nd client
func NewClient(host host.Host, opts ...Option) (*Client, error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(params)
	}

	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: client creation failed: %w", err)
	}

	return &Client{
		host:       host,
		protocolID: protocolID(params.protocolSuffix),
		params:     params,
	}, nil
}

// RequestND requests namespaced data from the given peer.
// Returns valid data with its verified inclusion against the share.Root.
func (c *Client) RequestND(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
	peer peer.ID,
) (share.NamespacedShares, error) {
	shares, err := c.doRequest(ctx, root, nID, peer)
	if err == nil {
		return shares, err
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
		log.Debugw("client-nd: peer returned err", "peer", peer, "err", err)
	}
	return nil, err
}

func (c *Client) doRequest(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
	peerID peer.ID,
) (share.NamespacedShares, error) {
	stream, err := c.host.NewStream(ctx, peerID, c.protocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	c.setStreamDeadlines(ctx, stream)

	req := &pb.GetSharesByNamespaceRequest{
		RootHash:    root.Hash(),
		NamespaceId: nID,
	}

	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("client-nd: writing request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Debugf("client-nd: closing write side of the stream: %s", err)
	}

	var resp pb.GetSharesByNamespaceResponse
	_, err = serde.Read(stream, &resp)
	if err != nil {
		// server is overloaded and closed the stream
		if errors.Is(err, io.EOF) {
			return nil, p2p.ErrUnavailable
		}
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("client-nd: reading response: %w", err)
	}

	if err = statusToErr(resp.Status); err != nil {
		return nil, fmt.Errorf("client-nd: response code is not OK: %w", err)
	}

	shares, err := convertToNamespacedShares(resp.Rows)
	if err != nil {
		return nil, fmt.Errorf("client-nd: converting response to shares: %w", err)
	}

	err = shares.Verify(root, nID)
	if err != nil {
		return nil, fmt.Errorf("client-nd: verifying response: %w", err)
	}

	return shares, nil
}

// convertToNamespacedShares converts proto Rows to share.NamespacedShares
func convertToNamespacedShares(rows []*pb.Row) (share.NamespacedShares, error) {
	shares := make([]share.NamespacedRow, 0, len(rows))
	for _, row := range rows {
		var proof *ipld.Proof
		if row.Proof != nil {
			cids := make([]cid.Cid, 0, len(row.Proof.Nodes))
			for _, node := range row.Proof.Nodes {
				cid, err := cid.Cast(node)
				if err != nil {
					return nil, fmt.Errorf("casting proofs node to cid: %w", err)
				}
				cids = append(cids, cid)
			}

			proof = &ipld.Proof{
				Nodes: cids,
				Start: int(row.Proof.Start),
				End:   int(row.Proof.End),
			}
		}

		shares = append(shares, share.NamespacedRow{
			Shares: row.Shares,
			Proof:  proof,
		})
	}
	return shares, nil
}

func (c *Client) setStreamDeadlines(ctx context.Context, stream network.Stream) {
	// set read/write deadline to use context deadline if it exists
	deadline, ok := ctx.Deadline()
	if ok {
		err := stream.SetDeadline(deadline)
		if err != nil {
			log.Debugf("client-nd: set write deadline: %s", err)
		}
		return
	}

	if c.params.readTimeout != 0 {
		err := stream.SetReadDeadline(time.Now().Add(c.params.readTimeout))
		if err != nil {
			log.Debugf("client-nd: set read deadline: %s", err)
		}
	}

	if c.params.writeTimeout != 0 {
		err := stream.SetWriteDeadline(time.Now().Add(c.params.readTimeout))
		if err != nil {
			log.Debugf("client-nd: set write deadline: %s", err)
		}
	}
}

func statusToErr(code pb.StatusCode) error {
	switch code {
	case pb.StatusCode_OK:
		return nil
	case pb.StatusCode_NOT_FOUND, pb.StatusCode_REFUSED:
		return p2p.ErrUnavailable
	case pb.StatusCode_INTERNAL, pb.StatusCode_INVALID:
		fallthrough
	default:
		log.Errorf("client-nd: request status %s returned", code.String())
		return p2p.ErrInvalidResponse
	}
}
