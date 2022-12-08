package availability

import (
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	"github.com/celestiaorg/nmt/namespace"
)

type Client struct {
	client *p2p.Client
}

func (c *Client) GetSharesWithProofs(
	ctx context.Context,
	height uint64,
	namespace namespace.ID,
	collectProofs bool,
	peerID peer.ID,
) ([]share.SharesWithProof, error) {
	var shares []share.SharesWithProof

	doRequest := func(cxt context.Context, session *p2p.Session) error {
		req := &share_p2p_v1.GetSharesByNamespaceRequest{
			Height:      int64(height),
			NamespaceId: namespace,
			WithProofs:  collectProofs,
		}
		err := session.Write(req)
		if err != nil {
			return err
		}

		var resp share_p2p_v1.GetSharesByNamespaceResponse
		err = session.Read(&resp)
		if err != nil {
			return err
		}

		if resp.Status != share_p2p_v1.StatusCode_OK {
			return fmt.Errorf("response code is not OK: %v", resp.Status)
		}

		shares, err = responseToShares(resp.Rows)
		return err
	}

	err := c.client.Do(ctx, peerID, protocolID, doRequest)
	return shares, err
}

// WIP
func responseToShares(rows []*share_p2p_v1.Row) ([]share.SharesWithProof, error) {
	shares := make([]share.SharesWithProof, 0, len(rows))
	for _, row := range rows {
		cids := make([]cid.Cid, 0, len(row.Proof.Nodes))
		for _, node := range row.Proof.Nodes {
			cid, err := cid.Cast(node)
			if err != nil {
				return nil, fmt.Errorf("cast proof node cid: %w", err)
			}
			cids = append(cids, cid)
		}

		shares = append(shares, share.SharesWithProof{
			Shares: row.Shares,
			Proof: &ipld.Proof{
				Nodes: cids,
				Start: int(row.Proof.Start),
				End:   int(row.Proof.End),
			},
		})
	}
	return shares, nil
}
