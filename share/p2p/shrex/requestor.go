package shrex

import (
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
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

type Client struct {
	host           host.Host
	defaultTimeout time.Duration
}

// GetSharesWithProofs gets shares with option to collect Merkle proofs from remote host
func (c *Client) GetSharesWithProofs(
	ctx context.Context,
	rootHash []byte,
	rowRoots [][]byte,
	maxShares int,
	nID namespace.ID,
	collectProofs bool,
	peerID peer.ID,
) ([]share.SharesWithProof, error) {
	// select rows that contain shares for given namespaceID
	selectedRoots := make([][]byte, 0)
	for _, row := range rowRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			selectedRoots = append(selectedRoots, row)
		}
	}
	if len(selectedRoots) == 0 {
		return nil, nil
	}

	stream, err := c.host.NewStream(ctx, peerID, ndProtocolID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := &share_p2p_v1.GetSharesByNamespaceRequest{
		RootHash:      rootHash,
		RowRoots:      selectedRoots,
		NamespaceId:   nID,
		MaxShares:     int64(maxShares),
		CollectProofs: collectProofs,
	}

	// if context doesn't have deadline use default one
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.defaultTimeout)
	}

	err = stream.SetWriteDeadline(deadline)
	if err != nil {
		log.Debugf("set write deadline: %s", err)
	}

	_, err = serde.Write(stream, req)
	if err != nil {
		return nil, fmt.Errorf("write req: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		log.Debugf("close write: %s", err)
	}

	err = stream.SetReadDeadline(deadline)
	if err != nil {
		log.Debugf("set read deadline: %s", err)
	}

	var resp share_p2p_v1.GetSharesByNamespaceResponse
	_, err = serde.Read(stream, &resp)
	if err != nil {
		return nil, fmt.Errorf("read resp: %w", err)
	}

	if resp.Status != share_p2p_v1.StatusCode_OK {
		return nil, fmt.Errorf("response code is not OK: %s", statusToString(resp.Status))
	}

	return responseToShares(resp.Rows)
}

// responseToShares converts proto Rows to SharesWithProof
func responseToShares(rows []*share_p2p_v1.Row) ([]share.SharesWithProof, error) {
	shares := make([]share.SharesWithProof, 0, len(rows))
	for _, row := range rows {
		var proof *ipld.Proof
		if row.Proof != nil {
			cids := make([]cid.Cid, 0, len(row.Proof.Nodes))
			for _, node := range row.Proof.Nodes {
				cid, err := cid.Cast(node)
				if err != nil {
					return nil, fmt.Errorf("cast proof node cid: %w", err)
				}
				cids = append(cids, cid)
			}

			proof = &ipld.Proof{
				Nodes: cids,
				Start: int(row.Proof.Start),
				End:   int(row.Proof.End),
			}
		}

		shares = append(shares, share.SharesWithProof{
			Shares: row.Shares,
			Proof:  proof,
		})
	}
	return shares, nil
}

func statusToString(code share_p2p_v1.StatusCode) string {
	switch code {
	case share_p2p_v1.StatusCode_INVALID:
		return "INVALID"
	case share_p2p_v1.StatusCode_OK:
		return "OK"
	case share_p2p_v1.StatusCode_NOT_FOUND:
		return "NOT_FOUND"
	case share_p2p_v1.StatusCode_INTERNAL:
		return "INTERNAL_ERR"
	}
	return ""
}
