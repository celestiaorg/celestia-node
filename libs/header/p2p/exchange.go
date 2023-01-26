package p2p

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	"github.com/celestiaorg/celestia-node/libs/header"
	p2p_pb "github.com/celestiaorg/celestia-node/libs/header/p2p/pb"
)

var log = logging.Logger("header/p2p")

// PubSubTopic hardcodes the name of the Header
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

	var (
		zero     H
		headerCh = make(chan H)
	)

	// request head from each trusted peer
	for _, from := range ex.trustedPeers {
		go func(from peer.ID) {
			headers, err := ex.request(ctx, from, req)
			if err != nil {
				log.Errorw("head request to trusted peer failed", "trustedPeer", from, "err", err)
				var zero H
				headerCh <- zero
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
			if !h.IsZero() {
				result = append(result, h)
			}
		case <-ctx.Done():
			break LOOP
		case <-ex.ctx.Done():
			return zero, ctx.Err()
		}
	}

	return bestHead[H](result, ex.Params.MinResponses)
}

// GetByHeight performs a request for the Header at the given
// height to the network. Note that the Header must be verified
// thereafter.
func (ex *Exchange[H]) GetByHeight(ctx context.Context, height uint64) (H, error) {
	log.Debugw("requesting header", "height", height)
	var zero H
	// sanity check height
	if height == 0 {
		return zero, fmt.Errorf("specified request height must be greater than 0")
	}
	// create request
	req := &p2p_pb.HeaderRequest{
		Data:   &p2p_pb.HeaderRequest_Origin{Origin: height},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return zero, err
	}
	return headers[0], nil
}

// GetRangeByHeight performs a request for the given range of Headers
// to the network. Note that the Headers must be verified thereafter.
func (ex *Exchange[H]) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]H, error) {
	if amount > ex.Params.MaxRequestSize {
		return nil, header.ErrHeadersLimitExceeded
	}
	session := newSession[H](ex.ctx, ex.host, ex.peerTracker, ex.protocolID, ex.Params.RequestTimeout)
	defer session.close()
	return session.getRangeByHeight(ctx, from, amount, ex.Params.MaxHeadersPerRequest)
}

// GetVerifiedRange performs a request for the given range of Headers to the network and
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

// Get performs a request for the Header by the given hash corresponding
// to the RawHeader. Note that the Header must be verified thereafter.
func (ex *Exchange[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	log.Debugw("requesting header", "hash", hash.String())
	var zero H
	// create request
	req := &p2p_pb.HeaderRequest{
		Data:   &p2p_pb.HeaderRequest_Hash{Hash: hash},
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return zero, err
	}

	if !bytes.Equal(headers[0].Hash(), hash) {
		return zero, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, headers[0].Hash())
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

	for ctx.Err() == nil {
		//nolint:gosec // G404: Use of weak random number generator
		index := rand.Intn(len(ex.trustedPeers))
		cctx, cancel := context.WithTimeout(context.Background(), time.Second)
		h, err := ex.request(cctx, ex.trustedPeers[index], req)
		cancel()
		if err != nil {
			log.Error(err)
			continue
		}
		return h, nil

	}

	return nil, ctx.Err()
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

// bestHead chooses Header that matches the conditions:
// * should have max height among received;
// * should be received at least from 2 peers;
// If neither condition is met, then latest Header will be returned (header of the highest
// height).
func bestHead[H header.Header](result []H, minResponses int) (H, error) {
	if len(result) == 0 {
		var zero H
		return zero, header.ErrNotFound
	}
	counter := make(map[string]int)
	// go through all of Headers and count the number of headers with a specific hash
	for _, res := range result {
		counter[res.Hash().String()]++
	}
	// sort results in a decreasing order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Height() > result[j].Height()
	})

	// try to find Header with the maximum height that was received at least from 2 peers
	for _, res := range result {
		if counter[res.Hash().String()] >= minResponses {
			return res, nil
		}
	}
	log.Debug("could not find latest header received from at least two peers, returning header with the max height")
	// otherwise return header with the max height
	return result[0], nil
}
