package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	p2p_pb "github.com/celestiaorg/celestia-node/share/eds/p2p/pb"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/rsmt2d"
)

var log = logging.Logger("shrex/eds")

var errNoMorePeers = errors.New("all peers returned invalid responses")

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/shrex/eds/v0.0.1/%s", protocolSuffix))
}

// Client is responsible for requesting EDSs for blocksync over the ShrEx/EDS protocol.
type Client struct {
	protocolID protocol.ID
	host       host.Host
}

// NewClient creates a new ShrEx/EDS client.
func NewClient(host host.Host, protocolSuffix string) *Client {
	return &Client{
		host:       host,
		protocolID: protocolID(protocolSuffix),
	}
}

// RequestEDS requests the full ODS from one of the given peers and returns the EDS.
//
// The peers are requested in a round-robin manner with retries until one of them gives a valid
// response, blocking until the context is canceled or a valid response is given.
func (c *Client) RequestEDS(
	ctx context.Context,
	dataHash share.DataHash,
	peers peer.IDSlice,
) (*rsmt2d.ExtendedDataSquare, error) {
	req := &p2p_pb.EDSRequest{Hash: dataHash}

	// requests are retried for every peer until a valid response is received
	excludedPeers := make(map[peer.ID]struct{})
	for {
		// if no peers are left, return
		if len(peers) == len(excludedPeers) {
			return nil, errNoMorePeers
		}

		for _, to := range peers {
			// skip over excluded peers
			if _, ok := excludedPeers[to]; ok {
				continue
			}
			eds, err := c.doRequest(ctx, req, to)
			if eds != nil {
				return eds, nil
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, ctx.Err()
			}
			// some net.Errors also mean the context deadline was exceeded, but yamux/mocknet do not
			// unwrap to a ctx err
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil, context.DeadlineExceeded
			}
			if err != nil {
				// peer has misbehaved, exclude them from round-robin
				excludedPeers[to] = struct{}{}
				log.Errorw("client: eds request to peer failed", "peer", to, "hash", dataHash.String())
			}

			// no eds was found, continue
		}
	}
}

func (c *Client) doRequest(
	ctx context.Context,
	req *p2p_pb.EDSRequest,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	dataHash := share.DataHash(req.Hash)
	log.Debugf("client: requesting eds %s from peer %s", dataHash.String(), to)
	stream, err := c.host.NewStream(ctx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		if err = stream.SetDeadline(dl); err != nil {
			log.Debugw("error setting deadline: %s", err)
		}
	}

	// request status from peer to provide the requested EDS
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to write request to stream: %w", err)
	}
	err = stream.CloseWrite()
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to close write on stream: %w", err)
	}

	// read and parse status from peer
	resp := new(p2p_pb.EDSResponse)
	_, err = serde.Read(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to read status from stream: %w", err)
	}

	switch resp.Status {
	case p2p_pb.Status_OK:
		// use header and ODS bytes to construct EDS and verify it against dataHash
		eds, err := eds.ReadEDS(ctx, stream, dataHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
		}

		return eds, nil
	case p2p_pb.Status_NOT_FOUND, p2p_pb.Status_REFUSED:
		log.Debugf("client: peer %s couldn't serve eds %s with status %s", to.String(), dataHash.String(), resp.GetStatus())
		// no eds was returned, but the request was valid and should be retried
		return nil, nil
	case p2p_pb.Status_INVALID:
		fallthrough
	default:
		return nil, fmt.Errorf("request status %s returned for root %s", resp.GetStatus(), dataHash.String())
	}
}
