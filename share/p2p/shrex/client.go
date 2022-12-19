package shrex

import (
	"errors"
	"fmt"

	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/nmt/namespace"
)

var (
	errInvalidStatusCode = errors.New("INVALID")
	errNotFound          = errors.New("NOT_FOUND")
	errServerInternal    = errors.New("SERVER_INTERNAL")
	errConnRefused       = errors.New("CONN_REFUSED")
	errNotAvailable      = errors.New("none of the peers has requested data")
)

// Client implements Getter interface, requesting data from remote peers
type Client struct {
	host           host.Host
	defaultTimeout time.Duration

	// fake peers for testing purposes
	testPeers []peer.ID
}

// NewClient creates a new shrEx client
func NewClient(host host.Host, defaultTimeout time.Duration) *Client {
	return &Client{
		host:           host,
		defaultTimeout: defaultTimeout,
	}
}

// GetBlobByNamespace request shares with option to collect proofs from remote peers using shrex protocol
func (c *Client) GetBlobByNamespace(ctx context.Context, root *share.Root, nID namespace.ID) (*share.Blob, error) {
	peers := c.testPeers
	if len(peers) == 0 { //nolint:staticcheck
		//TODO: collect peers from discovery here
	}

	for _, peer := range peers {
		blob, err := c.getBlobByNamespace(
			ctx, root, nID, peer)
		if err != nil {
			log.Debugw("peer returned err", "peer_id", peer.String(), "err", err)
			continue
		}

		return blob, nil
	}

	return nil, errNotAvailable
}

// getBlobByNamespace gets shares with Merkle tree inclusion proofs from remote host
func (c *Client) getBlobByNamespace(
	ctx context.Context,
	root *share.Root, nID namespace.ID,
	peerID peer.ID,
) (*share.Blob, error) {
	stream, err := c.host.NewStream(ctx, peerID, ndProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := &share_p2p_v1.GetSharesByNamespaceRequest{
		RootHash:    root.Hash(),
		NamespaceId: nID,
	}

	// if context doesn't have deadline use default one
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.defaultTimeout)
	}

	err = stream.SetDeadline(deadline)
	if err != nil {
		log.Debugf("set write deadline: %s", err)
	}

	_, err = serde.Write(stream, req)
	if err != nil {
		return nil, fmt.Errorf("write req: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Debugf("close write side of the stream: %s", err)
	}

	var resp share_p2p_v1.GetSharesByNamespaceResponse
	_, err = serde.Read(stream, &resp)
	if err != nil {
		return nil, fmt.Errorf("read resp: %w", err)
	}

	if err := statusToErr(resp.Status); err != nil {
		return nil, fmt.Errorf("response code is not OK: %w", err)
	}

	blob, err := responseToBlob(resp.Rows)
	if err != nil {
		return nil, fmt.Errorf("convert response to blob: %w", err)
	}

	err = blob.Verify(root, nID)
	if err != nil {
		return nil, err
	}

	return blob, nil
}

// responseToBlob converts proto Rows to share.Blob
func responseToBlob(rows []*share_p2p_v1.Row) (*share.Blob, error) {
	shares := make([]share.VerifiedShares, 0, len(rows))
	for _, row := range rows {
		var proof *ipld.Proof
		if row.Proof != nil {
			cids := make([]cid.Cid, 0, len(row.Proof.Nodes))
			for _, node := range row.Proof.Nodes {
				cid, err := cid.Cast(node)
				if err != nil {
					return nil, fmt.Errorf("cast proofs node to cid: %w", err)
				}
				cids = append(cids, cid)
			}

			proof = &ipld.Proof{
				Nodes: cids,
				Start: int(row.Proof.Start),
				End:   int(row.Proof.End),
			}
		}

		shares = append(shares, share.VerifiedShares{
			Shares: row.Shares,
			Proof:  proof,
		})
	}
	return &share.Blob{Rows: shares}, nil
}

func statusToErr(code share_p2p_v1.StatusCode) error {
	switch code {
	case share_p2p_v1.StatusCode_INVALID:
		return errInvalidStatusCode
	case share_p2p_v1.StatusCode_OK:
		return nil
	case share_p2p_v1.StatusCode_NOT_FOUND:
		return errNotFound
	case share_p2p_v1.StatusCode_INTERNAL:
		return errServerInternal
	case share_p2p_v1.StatusCode_REFUSED:
		return errConnRefused
	}
	return errInvalidStatusCode
}
