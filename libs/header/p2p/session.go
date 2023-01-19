package p2p

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/celestia-node/libs/header"
	p2p_pb "github.com/celestiaorg/celestia-node/libs/header/p2p/pb"
)

type option[H header.Header] func(*session[H])

func withValidation[H header.Header](from H) option[H] {
	return func(s *session[H]) {
		s.from = from
	}
}

// session aims to divide a range of headers
// into several smaller requests among different peers.
type session[H header.Header] struct {
	host       host.Host
	protocolID protocol.ID
	queue      *peerQueue
	// peerTracker contains discovered peers with records that describes their activity.
	peerTracker *peerTracker

	// `from` is set when additional validation for range is needed.
	// Otherwise, it will be nil.
	from           H
	requestTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	reqCh  chan *p2p_pb.HeaderRequest
}

func newSession[H header.Header](
	ctx context.Context,
	h host.Host,
	peerTracker *peerTracker,
	protocolID protocol.ID,
	requestTimeout time.Duration,
	options ...option[H],
) *session[H] {
	ctx, cancel := context.WithCancel(ctx)
	ses := &session[H]{
		ctx:            ctx,
		cancel:         cancel,
		protocolID:     protocolID,
		host:           h,
		queue:          newPeerQueue(ctx, peerTracker.peers()),
		peerTracker:    peerTracker,
		requestTimeout: requestTimeout,
	}

	for _, opt := range options {
		opt(ses)
	}
	return ses
}

// GetRangeByHeight requests headers from different peers.
func (s *session[H]) getRangeByHeight(
	ctx context.Context,
	from, amount, headersPerPeer uint64,
) ([]H, error) {
	log.Debugw("requesting headers", "from", from, "to", from+amount)

	requests := prepareRequests(from, amount, headersPerPeer)
	result := make(chan []H, len(requests))
	s.reqCh = make(chan *p2p_pb.HeaderRequest, len(requests))

	go s.handleOutgoingRequests(ctx, result)
	for _, req := range requests {
		s.reqCh <- req
	}

	headers := make([]H, 0, amount)
	for i := 0; i < len(requests); i++ {
		select {
		case <-s.ctx.Done():
			return nil, errors.New("header/p2p: exchange is closed")
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-result:
			headers = append(headers, res...)
		}
	}
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height() < headers[j].Height()
	})
	return headers, nil
}

// close stops the session.
func (s *session[H]) close() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// handleOutgoingRequests pops a peer from the queue and sends a prepared request to the peer.
// Will exit via canceled session context or when all request are processed.
func (s *session[H]) handleOutgoingRequests(ctx context.Context, result chan []H) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case req := <-s.reqCh:
			// select peer with the highest score among the available ones for the request
			stats := s.queue.waitPop(ctx)
			if stats.peerID == "" {
				return
			}
			go s.doRequest(ctx, stats, req, result)
		}
	}
}

// doRequest chooses the best peer to fetch headers and sends a request in range of available
// maxRetryAttempts.
func (s *session[H]) doRequest(
	ctx context.Context,
	stat *peerStat,
	req *p2p_pb.HeaderRequest,
	headers chan []H,
) {
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	r, size, duration, err := sendMessage(ctx, s.host, stat.peerID, s.protocolID, req)
	if err != nil {
		log.Errorw("requesting headers from peer failed.", "failed peer", stat.peerID, "err", err)
		select {
		case <-s.ctx.Done():
		case s.reqCh <- req:
			stat.decreaseScore()
			log.Debug("Retrying the request from different peer")
		}
		return
	}

	h, err := s.processResponse(r)
	if err != nil {
		switch err {
		case header.ErrNotFound:
		default:
			s.peerTracker.blockPeer(stat.peerID, err)
		}
		select {
		case <-s.ctx.Done():
		case s.reqCh <- req:
		}
		return
	}
	log.Debugw("request headers from peer succeed ", "from", s.host.ID(), "pID", stat.peerID, "amount", req.Amount)
	// send headers to the channel, update peer stats and return peer to the queue, so it can be
	// re-used in case if there are other requests awaiting
	headers <- h
	stat.updateStats(size, duration)
	s.queue.push(stat)
}

// processResponse converts HeaderResponse to Header.
func (s *session[H]) processResponse(responses []*p2p_pb.HeaderResponse) ([]H, error) {
	headers := make([]H, 0)
	for _, resp := range responses {
		err := convertStatusCodeToError(resp.StatusCode)
		if err != nil {
			return nil, err
		}
		var empty H
		header := empty.New()
		err = header.UnmarshalBinary(resp.Body)
		if err != nil {
			return nil, errors.New("unmarshalling error")
		}
		headers = append(headers, header.(H))
	}
	if len(headers) == 0 {
		return nil, header.ErrNotFound
	}

	err := s.validate(headers)
	return headers, err
}

// validate checks that the received range of headers is valid against the provided header.
func (s *session[H]) validate(headers []H) error {
	// if `from` is empty, then additional validation for the header`s range is not needed.
	if s.from.IsZero() {
		return nil
	}

	// verify that the first header in range is valid against the trusted header.
	err := s.from.VerifyNonAdjacent(headers[0])
	if err != nil {
		return nil
	}

	if len(headers) == 1 {
		return nil
	}

	trusted := headers[0]
	// verify that the whole range is valid.
	for i := 1; i < len(headers); i++ {
		err = trusted.VerifyAdjacent(headers[i])
		if err != nil {
			return err
		}
		trusted = headers[i]
	}
	return nil
}

// prepareRequests converts incoming range into separate HeaderRequest.
func prepareRequests(from, amount, headersPerPeer uint64) []*p2p_pb.HeaderRequest {
	requests := make([]*p2p_pb.HeaderRequest, 0, amount/headersPerPeer)
	for amount > uint64(0) {
		var requestSize uint64
		request := &p2p_pb.HeaderRequest{
			Data: &p2p_pb.HeaderRequest_Origin{Origin: from},
		}
		if amount < headersPerPeer {
			requestSize = amount
			amount = 0
		} else {
			amount -= headersPerPeer
			from += headersPerPeer
			requestSize = headersPerPeer
		}
		request.Amount = requestSize
		requests = append(requests, request)
	}
	return requests
}
