package p2p

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sort"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	"github.com/celestiaorg/celestia-node/libs/header"
	p2p_pb "github.com/celestiaorg/celestia-node/libs/header/p2p/pb"
)

var log = logging.Logger("header/p2p")

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// Exchange enables sending outbound HeaderRequests to the network as well as
// handling inbound HeaderRequests from the network.
type Exchange[H header.Header] struct {
	ctx    context.Context
	cancel context.CancelFunc

	protocolID protocol.ID
	host       host.Host

	trustedPeers peer.IDSlice
	peerTracker  *peerTracker

	Params ClientParameters
}

func NewExchange[H header.Header](
	host host.Host,
	peers peer.IDSlice,
	protocolSuffix string,
	connGater *conngater.BasicConnectionGater,
	opts ...Option[ClientParameters],
) (*Exchange[H], error) {
	params := DefaultClientParameters()
	for _, opt := range opts {
		opt(&params)
	}

	err := params.Validate()
	if err != nil {
		return nil, err
	}

	return &Exchange[H]{
		host:         host,
		protocolID:   protocolID(protocolSuffix),
		trustedPeers: peers,
		peerTracker: newPeerTracker(
			host,
			connGater,
			params.MaxAwaitingTime,
			params.DefaultScore,
			params.MaxPeerTrackerSize,
		),
		Params: params,
	}, nil
}

func (ex *Exchange[H]) Start(context.Context) error {
	ex.ctx, ex.cancel = context.WithCancel(context.Background())

	for _, p := range ex.trustedPeers {
		// Try to pre-connect to trusted peers.
		// We don't really care if we succeed at this point
		// and just need any peers in the peerTracker asap
		go ex.host.Connect(ex.ctx, peer.AddrInfo{ID: p}) //nolint:errcheck
	}
	go ex.peerTracker.gc()
	go ex.peerTracker.track()
	return nil
}

func (ex *Exchange[H]) Stop(context.Context) error {
	// cancel the session if it exists
	ex.cancel()
	// stop the peerTracker
	ex.peerTracker.stop()
	return nil
}

// Head requests the latest Header. Note that the Header must be verified thereafter.
// NOTE:
// It is fine to continue handling head request if the timeout will be reached.
// As we are requesting head from multiple trusted peers,
// we may already have some headers when the timeout will be reached.
func (ex *Exchange[H]) Head(ctx context.Context) (H, error) {
	log.Debug("requesting head")
	// create request
	req := &p2p_pb.HeaderRequest{
		Data:   &p2p_pb.HeaderRequest_Origin{Origin: uint64(0)},
		Amount: 1,
	}

	headerCh := make(chan H)
	// request head from each trusted peer
	for _, from := range ex.trustedPeers {
		go func(from peer.ID) {
			headers, err := ex.request(ctx, from, req)
			if err != nil {
				log.Errorw("head request to trusted peer failed", "trustedPeer", from, "err", err)
				headerCh <- *new(H) //nolint:gocritic
				return
			}
			// doRequest ensures that the result slice will have at least one Header
			headerCh <- headers[0]
		}(from)
	}

	result := make([]H, 0, len(ex.trustedPeers))
LOOP:
	for range ex.trustedPeers {
		select {
		case h := <-headerCh:
			if header.Header(h) != header.Header(*new(H)) { //nolint:gocritic
				result = append(result, h)
			}
		case <-ctx.Done():
			break LOOP
		case <-ex.ctx.Done():
			return *new(H), ctx.Err()
		}
	}

	return bestHead[H](result, ex.Params.MinResponses)
}

// GetByHeight performs a request for the ExtendedHeader at the given
// height to the network. Note that the ExtendedHeader must be verified
// thereafter.
func (ex *Exchange[H]) GetByHeight(ctx context.Context, height uint64) (H, error) {
	log.Debugw("requesting header", "height", height)
	// sanity check height
	if height == 0 {
		return *new(H), fmt.Errorf("specified request height must be greater than 0") //nolint:gocritic
	}
	// create request
	req := &p2p_pb.HeaderRequest{
		Data:   &p2p_pb.HeaderRequest_Origin{Origin: height},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return *new(H), err //nolint:gocritic
	}
	return headers[0], nil
}

// GetRangeByHeight performs a request for the given range of ExtendedHeaders
// to the network. Note that the ExtendedHeaders must be verified thereafter.
func (ex *Exchange[H]) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]H, error) {
	if amount > ex.Params.MaxRequestSize {
		return nil, header.ErrHeadersLimitExceeded
	}
	session := newSession[H](ex.ctx, ex.host, ex.peerTracker, ex.protocolID, ex.Params.RequestTimeout)
	defer session.close()
	return session.getRangeByHeight(ctx, from, amount, ex.Params.MaxHeadersPerRequest)
}

// GetVerifiedRange performs a request for the given range of ExtendedHeaders to the network and
// ensures that returned headers are correct against the passed one.
func (ex *Exchange[H]) GetVerifiedRange(
	ctx context.Context,
	from H,
	amount uint64,
) ([]H, error) {
	session := newSession[H](
		ex.ctx, ex.host, ex.peerTracker, ex.protocolID, ex.Params.RequestTimeout, withValidation(from),
	)
	defer session.close()

	return session.getRangeByHeight(ctx, uint64(from.Height())+1, amount, ex.Params.MaxHeadersPerRequest)
}

// Get performs a request for the ExtendedHeader by the given hash corresponding
// to the RawHeader. Note that the ExtendedHeader must be verified thereafter.
func (ex *Exchange[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	log.Debugw("requesting header", "hash", hash.String())
	// create request
	req := &p2p_pb.HeaderRequest{
		Data:   &p2p_pb.HeaderRequest_Hash{Hash: hash},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return *new(H), err //nolint:gocritic
	}

	if !bytes.Equal(headers[0].Hash(), hash) {
		return *new(H), fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, headers[0].Hash()) //nolint:gocritic
	}
	return headers[0], nil
}

func (ex *Exchange[H]) performRequest(
	ctx context.Context,
	req *p2p_pb.HeaderRequest,
) ([]H, error) {
	if req.Amount == 0 {
		return make([]H, 0), nil
	}

	if len(ex.trustedPeers) == 0 {
		return nil, fmt.Errorf("no trusted peers")
	}

	//nolint:gosec // G404: Use of weak random number generator
	index := rand.Intn(len(ex.trustedPeers))
	return ex.request(ctx, ex.trustedPeers[index], req)
}

// request sends the HeaderRequest to a remote peer.
func (ex *Exchange[H]) request(
	ctx context.Context,
	to peer.ID,
	req *p2p_pb.HeaderRequest,
) ([]H, error) {
	log.Debugw("requesting peer", "peer", to)
	responses, _, _, err := sendMessage(ctx, ex.host, to, ex.protocolID, req)
	if err != nil {
		log.Debugw("err sending request", "peer", to, "err", err)
		return nil, err
	}
	if len(responses) == 0 {
		return nil, header.ErrNotFound
	}
	headers := make([]H, 0, len(responses))
	for _, response := range responses {
		if err = convertStatusCodeToError(response.StatusCode); err != nil {
			return nil, err
		}
		var empty H
		header := empty.New()
		err := header.UnmarshalBinary(response.Body)
		if err != nil {
			return nil, err
		}
		headers = append(headers, header.(H))
	}
	return headers, nil
}

// bestHead chooses ExtendedHeader that matches the conditions:
// * should have max height among received;
// * should be received at least from 2 peers;
// If neither condition is met, then latest ExtendedHeader will be returned (header of the highest
// height).
func bestHead[H header.Header](result []H, minResponses int) (H, error) {
	if len(result) == 0 {
		return *new(H), header.ErrNotFound //nolint:gocritic
	}
	counter := make(map[string]int)
	// go through all of ExtendedHeaders and count the number of headers with a specific hash
	for _, res := range result {
		counter[res.Hash().String()]++
	}
	// sort results in a decreasing order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Height() > result[j].Height()
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
