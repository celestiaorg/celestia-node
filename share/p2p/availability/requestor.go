package availability

import (
	"fmt"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"golang.org/x/net/context"
)

type requestor struct {
	client p2p.Client
}

func (c *requestor) GetSharesWithProofs(
	ctx context.Context,
	height uint64,
	namespace namespace.ID,
	proofs *nmt.Proof,
) ([]share.Share, error) {
	session, err := c.client.Do(ctx, protocolID)
	if err != nil {
		return nil, err
	}

	// collect proofs if providede proof container is not nil
	withProofs := proofs != nil
	req := &share_p2p_v1.GetSharesByNamespaceRequest{
		Height:      int64(height),
		NamespaceId: namespace,
		WithProofs:  withProofs,
	}
	err = session.Write(req)
	if err != nil {
		return nil, err
	}

	resp := new(share_p2p_v1.GetSharesByNamespaceResponse)
	err = session.Read(resp)
	if err != nil {
		return nil, err
	}

	if resp.Status != share_p2p_v1.StatusCode_OK {
		return nil, fmt.Errorf("response code is not OK: %v", resp.Status)
	}

	return responseToShares(resp), nil
}

// WIP
func responseToShares(*share_p2p_v1.GetSharesByNamespaceResponse) []share.Share {
	return nil
}
