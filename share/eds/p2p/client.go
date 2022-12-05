package p2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

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
func (c *Client) RequestEDS(
	ctx context.Context,
	root share.Root,
	peers peer.IDSlice,
) (*rsmt2d.ExtendedDataSquare, error) {
	req := &p2p_pb.EDSRequest{Hash: root.Hash()}

	for _, to := range peers {
		log.Debugf("client: requesting eds %s from peer %s", root.Hash(), to)
		stream, err := c.host.NewStream(ctx, to, c.protocolID)
		if err != nil {
			return nil, fmt.Errorf("client: failed to open stream with peer %s: %w", to, err)
		}

		if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
			log.Warn(err)
		}
		_, err = serde.Write(stream, req)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, fmt.Errorf("client: failed to write request to stream with peer %s: %w", to, err)
		}

		err = stream.CloseWrite()
		if err != nil {
			return nil, fmt.Errorf("client: failed to close write on stream with peer %s: %w", to, err)
		}

		resp := new(p2p_pb.EDSResponse)
		if err := stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			log.Warn(err)
		}
		_, err = serde.Read(stream, resp)
		if err != nil {
			// TODO(@distractedm1nd): is this EOF check really necessary here?
			if err == io.EOF {
				break
			}
			stream.Reset() //nolint:errcheck
			return nil, fmt.Errorf("client: failed to read status from stream %s: %w", to, err)
		}

		switch resp.Status {
		case p2p_pb.Status_OK:
			odsBytes, err := io.ReadAll(stream)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("client: unexpected error while reading ods from stream: %w", err)
			}
			carReader := bytes.NewReader(odsBytes)
			eds, err := eds.ReadEDS(ctx, carReader, root)
			if err != nil {
				return nil, fmt.Errorf("client: failed to read eds from ods bytes: %w", err)
			}

			return eds, nil
		case p2p_pb.Status_NOT_FOUND, p2p_pb.Status_REFUSED:
			log.Debug("client: peer %s refused to serve eds %s with status", to, root.Hash(), resp.Status)
		case p2p_pb.Status_INVALID:
			log.Errorf("client: peer %s marked your request for root %s as invalid", to, root.Hash())
		default:
			log.Errorf("client: peer %s returned unimplemented status: %s", to, resp.Status)
		}
	}

	return nil, fmt.Errorf("client: no successful response was attained by any of the given peers")
}
