package p2p

import (
	"bytes"
	"context"
	"errors"
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

var errNoMorePeers = errors.New("all peers returned invalid responses")

// TODO(@distractedm1nd): make version suffix configurable
var protocolID = protocol.ID("/shrex/eds/0.0.1")

// Client is responsible for requesting EDSs for blocksync over the ShrEx/EDS protocol.
// This client is run by light nodes and full nodes. For more information, see ADR #13
type Client struct {
	protocolID protocol.ID
	host       host.Host
}

// NewClient creates a new ShrEx/EDS client.
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
	dataHash share.DataHash,
	peers peer.IDSlice,
) (*rsmt2d.ExtendedDataSquare, error) {
	req := &p2p_pb.EDSRequest{Hash: dataHash}

	// requests are retried for every peer until a valid response is received
	edsCh := make(chan *rsmt2d.ExtendedDataSquare)
	reqContext, cancel := context.WithCancel(context.Background())
	go func() {
		// cancel all requests once a valid response is received
		defer cancel()

		excludedPeers := make(map[peer.ID]struct{})
		for {
			// if no peers are left, return
			if len(peers) == len(excludedPeers) {
				return
			}

			for _, to := range peers {
				// skip over excluded peers
				if _, ok := excludedPeers[to]; ok {
					continue
				}

				eds, err := c.doRequest(reqContext, req, dataHash, to)
				if err != nil {
					// peer has misbehaved, exclude them from round-robin
					excludedPeers[to] = struct{}{}
					log.Errorw("client: eds request to peer failed", "peer", to, "hash", dataHash.String())
				}

				// eds is nil when the request was valid but couldn't be served
				if eds != nil {
					edsCh <- eds
					return
				}
			}
		}
	}()

	for {
		select {
		case eds := <-edsCh:
			return eds, nil
		case <-reqContext.Done():
			return nil, errNoMorePeers
		case <-ctx.Done():
			// no response was received before the context was canceled
			cancel()
			return nil, ctx.Err()
		}
	}
}

func (c *Client) doRequest(
	ctx context.Context,
	req *p2p_pb.EDSRequest,
	dataHash share.DataHash,
	to peer.ID,
) (*rsmt2d.ExtendedDataSquare, error) {
	log.Debugf("client: requesting eds %s from peer %s", dataHash.String(), to)
	stream, err := c.host.NewStream(ctx, to, c.protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	err = stream.SetWriteDeadline(time.Now().Add(writeDeadline))
	if err != nil {
		log.Warn(err)
	}
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to write request to stream: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		return nil, fmt.Errorf("failed to close write on stream: %w", err)
	}

	resp := new(p2p_pb.EDSResponse)
	err = stream.SetReadDeadline(time.Now().Add(readDeadline))
	if err != nil {
		log.Warn(err)
	}
	_, err = serde.Read(stream, resp)
	if err != nil && err != io.EOF {
		stream.Reset() //nolint:errcheck
		return nil, fmt.Errorf("failed to read status from stream: %w", err)
	}

	switch resp.Status {
	case p2p_pb.Status_OK:
		odsBytes, err := io.ReadAll(stream)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("unexpected error while reading ods from stream: %w", err)
		}
		carReader := bytes.NewReader(odsBytes)
		eds, err := eds.ReadEDS(ctx, carReader, dataHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read eds from ods bytes: %w", err)
		}

		return eds, nil

	case p2p_pb.Status_NOT_FOUND, p2p_pb.Status_REFUSED:
		log.Debug("client: peer %s refused to serve eds %s with status", to, dataHash.String(), resp.GetStatus())
	case p2p_pb.Status_INVALID:
		// TODO: if a peer marks a request as invalid, should we not request from them again?
		return nil, fmt.Errorf("request for root %s marked as invalid by peer", dataHash.String())
	}

	// no eds was returned, but the request was valid and should be retried
	return nil, nil
}
