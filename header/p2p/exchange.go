package p2p

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
)

var log = logging.Logger("header/p2p")

const (
	// writeDeadline sets timeout for sending messages to the stream
	writeDeadline = time.Second * 5
	// readDeadline sets timeout for reading messages from the stream
	readDeadline = time.Minute
	// the target minimum amount of responses with the same chain head
	minResponses = 2
	// requestSize defines the max amount of headers that can be requested/handled at once.
	maxRequestSize uint64 = 512
)

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// Exchange enables sending outbound ExtendedHeaderRequests to the network as well as
// handling inbound ExtendedHeaderRequests from the network.
type Exchange struct {
	protocolID protocol.ID

	host host.Host

	trustedPeers peer.IDSlice
}

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/header-ex/v0.0.3/%s", protocolSuffix))
}

func NewExchange(host host.Host, peers peer.IDSlice, protocolSuffix string) *Exchange {
	return &Exchange{
		host:         host,
		protocolID:   protocolID(protocolSuffix),
		trustedPeers: peers,
	}
}

// Head requests the latest ExtendedHeader. Note that the ExtendedHeader
// must be verified thereafter.
// NOTE:
// It is fine to continue handling head request if the timeout will be reached.
// As we are requesting head from multiple trusted peers,
// we may already have some headers when the timeout will be reached.
func (ex *Exchange) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	log.Debug("requesting head")
	// create request
	req := &p2p_pb.ExtendedHeaderRequest{
		Data:   &p2p_pb.ExtendedHeaderRequest_Origin{Origin: uint64(0)},
		Amount: 1,
	}

	headerCh := make(chan *header.ExtendedHeader)
	// request head from each trusted peer
	for _, from := range ex.trustedPeers {
		go func(from peer.ID) {
			headers, err := ex.request(ctx, from, req)
			if err != nil {
				log.Errorw("head request to trusted peer failed", "trustedPeer", from, "err", err)
				headerCh <- nil
				return
			}
			// doRequest ensures that the result slice will have at least one ExtendedHeader
			headerCh <- headers[0]
		}(from)
	}

	result := make([]*header.ExtendedHeader, 0, len(ex.trustedPeers))
LOOP:
	for range ex.trustedPeers {
		select {
		case h := <-headerCh:
			if h != nil {
				result = append(result, h)
			}
		case <-ctx.Done():
			break LOOP
		}
	}

	return bestHead(result)
}

// GetByHeight performs a request for the ExtendedHeader at the given
// height to the network. Note that the ExtendedHeader must be verified
// thereafter.
func (ex *Exchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "height", height)
	// sanity check height
	if height == 0 {
		return nil, fmt.Errorf("specified request height must be greater than 0")
	}
	// create request
	req := &p2p_pb.ExtendedHeaderRequest{
		Data:   &p2p_pb.ExtendedHeaderRequest_Origin{Origin: height},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return headers[0], nil
}

// GetRangeByHeight performs a request for the given range of ExtendedHeaders
// to the network. Note that the ExtendedHeaders must be verified thereafter.
func (ex *Exchange) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]*header.ExtendedHeader, error) {
	log.Debugw("requesting headers", "from", from, "to", from+amount)
	// create request
	req := &p2p_pb.ExtendedHeaderRequest{
		Data:   &p2p_pb.ExtendedHeaderRequest_Origin{Origin: from},
		Amount: amount,
	}
	return ex.performRequest(ctx, req)
}

// Get performs a request for the ExtendedHeader by the given hash corresponding
// to the RawHeader. Note that the ExtendedHeader must be verified thereafter.
func (ex *Exchange) Get(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	log.Debugw("requesting header", "hash", hash.String())
	// create request
	req := &p2p_pb.ExtendedHeaderRequest{
		Data:   &p2p_pb.ExtendedHeaderRequest_Hash{Hash: hash.Bytes()},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(headers[0].Hash().Bytes(), hash) {
		return nil, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, headers[0].Hash().Bytes())
	}
	return headers[0], nil
}

func (ex *Exchange) performRequest(
	ctx context.Context,
	req *p2p_pb.ExtendedHeaderRequest,
) ([]*header.ExtendedHeader, error) {
	if req.Amount == 0 {
		return make([]*header.ExtendedHeader, 0), nil
	}

	if len(ex.trustedPeers) == 0 {
		return nil, fmt.Errorf("no trusted peers")
	}

	//nolint:gosec // G404: Use of weak random number generator
	index := rand.Intn(len(ex.trustedPeers))
	return ex.request(ctx, ex.trustedPeers[index], req)
}

// request sends the ExtendedHeaderRequest to a remote peer.
func (ex *Exchange) request(
	ctx context.Context,
	to peer.ID,
	req *p2p_pb.ExtendedHeaderRequest,
) ([]*header.ExtendedHeader, error) {
	stream, err := ex.host.NewStream(ctx, to, ex.protocolID)
	if err != nil {
		return nil, err
	}
	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Debugf("error setting deadline: %s", err)
	}
	// send request
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	err = stream.CloseWrite()
	if err != nil {
		log.Error(err)
	}
	// read responses
	headers := make([]*header.ExtendedHeader, req.Amount)
	for i := 0; i < int(req.Amount); i++ {
		resp := new(p2p_pb.ExtendedHeaderResponse)
		if err = stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			log.Debugf("error setting deadline: %s", err)
		}
		_, err := serde.Read(stream, resp)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		if err = convertStatusCodeToError(resp.StatusCode); err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}
		header, err := header.UnmarshalExtendedHeader(resp.Body)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		headers[i] = header
	}
	if err = stream.Close(); err != nil {
		log.Errorw("closing stream", "err", err)
	}
	if len(headers) == 0 {
		return nil, header.ErrNotFound
	}
	return headers, nil
}

// bestHead chooses ExtendedHeader that matches the conditions:
// * should have max height among received;
// * should be received at least from 2 peers;
// If neither condition is met, then latest ExtendedHeader will be returned (header of the highest
// height).
func bestHead(result []*header.ExtendedHeader) (*header.ExtendedHeader, error) {
	if len(result) == 0 {
		return nil, header.ErrNotFound
	}
	counter := make(map[string]int)
	// go through all of ExtendedHeaders and count the number of headers with a specific hash
	for _, res := range result {
		counter[res.Hash().String()]++
	}
	// sort results in a decreasing order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Height > result[j].Height
	})

	// try to find ExtendedHeader with the maximum height that was received at least from 2 peers
	for _, res := range result {
		if counter[res.Hash().String()] >= minResponses {
			return res, nil
		}
	}
	log.Debug("could not find latest header received from at least two peers, returning header with the max height")
	// otherwise return header with the max height
	return result[0], nil
}

// convertStatusCodeToError converts passed status code into an error.
func convertStatusCodeToError(code p2p_pb.StatusCode) error {
	switch code {
	case p2p_pb.StatusCode_OK:
		return nil
	case p2p_pb.StatusCode_NOT_FOUND:
		return header.ErrNotFound
	case p2p_pb.StatusCode_LIMIT_EXCEEDED:
		return header.ErrHeadersLimitExceeded
	default:
		return fmt.Errorf("unknown status code %d", code)
	}
}
