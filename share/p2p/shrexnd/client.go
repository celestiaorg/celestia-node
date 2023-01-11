package shrexnd

import (
	"errors"
	"fmt"

	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/nmt/namespace"
)

// Client implements client side of shrex/nd protocol to obtain namespaced shares data from remote
// peers.
type Client struct {
	host    host.Host
	timeout time.Duration
}

// NewClient creates a new shrEx/nd client
func NewClient(host host.Host, timeout time.Duration) *Client {
	return &Client{
		host:    host,
		timeout: timeout,
	}
}

// GetSharesByNamespace request shares with option to collect proofs from remote peers using shrex
// protocol
func (c *Client) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
	peerID peer.ID,
) (share.NamespacedShares, error) {
	stream, err := c.host.NewStream(ctx, peerID, ndProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := &pb.GetSharesByNamespaceRequest{
		RootHash:    root.Hash(),
		NamespaceId: nID,
	}

	// if context doesn't have deadline use default one
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
	}

	err = stream.SetDeadline(deadline)
	if err != nil {
		log.Debugf("set write deadline: %s", err)
	}

	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("client: writing request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Debugf("client: closing write side of the stream: %s", err)
	}

	var resp pb.GetSharesByNamespaceResponse
	_, err = serde.Read(stream, &resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("client: reading response: %w", err)
	}

	if err = statusToErr(resp.Status); err != nil {
		return nil, fmt.Errorf("client: response code is not OK: %w", err)
	}

	shares, err := responseToNamespacedShares(resp.Rows)
	if err != nil {
		return nil, fmt.Errorf("client: converting response to shares: %w", err)
	}

	err = shares.Verify(root, nID)
	if err != nil {
		return nil, fmt.Errorf("client: verifing response: %w", err)
	}

	return shares, nil
}

// responseToNamespacedShares converts proto Rows to share.NamespacedShares
func responseToNamespacedShares(rows []*pb.Row) (share.NamespacedShares, error) {
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
