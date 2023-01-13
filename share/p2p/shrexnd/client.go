package shrexnd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/nmt/namespace"
)

var errNoMorePeers = errors.New("shrex-nd: all peers returned invalid responses")

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

// GetSharesByNamespace request shares with option to collect proofs from remote peers using shrex
// protocol
func (c *Client) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
	peerIDs ...peer.ID,
) (share.NamespacedShares, error) {
	for _, peerID := range peerIDs {
		shares, err := c.doRequest(ctx, root, nID, peerID)
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
				// stop trying peers if ctx deadline reached
				return nil, context.DeadlineExceeded
			}
		}

		// log and try another peer
		log.Errorw("client-nd: peer returned err", "peer_id", peerID.String(), "err", err)
	}

	return nil, errNoMorePeers
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
	case pb.StatusCode_INVALID,
		pb.StatusCode_NOT_FOUND,
		pb.StatusCode_INTERNAL,
		pb.StatusCode_REFUSED:
	default:
		code = pb.StatusCode_INVALID
	}
	return errors.New(code.String())
}
