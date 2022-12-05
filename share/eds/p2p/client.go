package p2p

import (
	"bytes"
	"context"
	"fmt"
	"io"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	p2p_pb "github.com/celestiaorg/celestia-node/share/eds/p2p/pb"

	"github.com/celestiaorg/rsmt2d"
)

var log = logging.Logger("shrex/eds")

// TODO(@distractedm1nd): make version suffix configurable
var protocolID = protocol.ID("/shrex/eds/0.0.1")

type Client struct {
	protocolID protocol.ID
	host       host.Host
}

func NewClient(host host.Host) *Client {
	return &Client{
		host:       host,
		protocolID: protocolID,
	}
}

// RequestEDS requests the full EDS from one of the given peers.
//
// The peers are requested in a round-robin manner with retries until one of them gives a valid
// response. Blocks forever until the context is canceled or a valid response is given.
//
// TODO(@distractedm1nd): add read/write deadlines, wrapp errors better, more logging
func (c *Client) RequestEDS(
	ctx context.Context,
	root share.Root,
	peers peer.IDSlice,
) (*rsmt2d.ExtendedDataSquare, error) {
	req := &p2p_pb.EDSRequest{Hash: root.Hash()}

	for _, to := range peers {
		stream, err := c.host.NewStream(ctx, to, c.protocolID)
		if err != nil {
			return nil, err
		}

		_, err = serde.Write(stream, req)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		err = stream.CloseWrite()
		if err != nil {
			return nil, err
		}

		resp := new(p2p_pb.EDSResponse)
		_, err = serde.Read(stream, resp)
		if err != nil {
			// TODO(@distractedm1nd): is this EOF check really necessary here?
			if err == io.EOF {
				break
			}
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		switch resp.Status {
		case p2p_pb.Status_OK:
			odsBytes, err := io.ReadAll(stream)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("unexpected error while reading ods from stream: %w", err)
			}
			carReader := bytes.NewReader(odsBytes)
			eds, err := eds.ReadEDS(ctx, carReader, root)
			if err != nil {
				return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
			}

			return eds, nil
		case p2p_pb.Status_NOT_FOUND, p2p_pb.Status_REFUSED, p2p_pb.Status_INVALID:
			fallthrough
		default:
			log.Errorf("peer %s returned unimplemented status: %s", to, resp.Status)
		}
	}

	return nil, fmt.Errorf("no successful response was attained by any of the given peers")
}
