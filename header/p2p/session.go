package p2p

import (
	"context"
	"errors"
	"time"
	"unsafe"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
)

const (
	// chunkSize is a maximum amount of headers that will be requested per peer
	chunkSize uint64 = 64
)

// session aims to divide a range of headers into several chunks and to request this chunks from different peers.
type session struct {
	ctx    context.Context
	cancel context.CancelFunc
	host   host.Host
	// peerTracker contains discovered peer with records that describes peer's activity.
	peerTracker *peerTracker
	// maxRetryAttempts is amount of retry attempts per each request.
	maxRetryAttempts int

	errCh chan error
}

func newSession(h host.Host, tracker *peerTracker) *session {
	ctx, cancel := context.WithCancel(context.Background())
	return &session{
		ctx:              ctx,
		cancel:           cancel,
		host:             h,
		peerTracker:      tracker,
		maxRetryAttempts: 3,
		errCh:            make(chan error),
	}
}

// doRequest chooses the best peer to fetch headers and sends a request in range of available maxRetryAttempts
func (s *session) doRequest(ctx context.Context, req *p2p_pb.ExtendedHeaderRequest, headers chan []*header.ExtendedHeader) {
	for i := 0; i < s.maxRetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		default:
		}
		// find the best peer from the peerTracker
		peerID := s.peerTracker.calculateBestPeer()

		// request headers from the remote peer
		r, size, duration, err := s.requestHeaders(ctx, peerID, req)
		if err != nil {
			s.peerTracker.logFail(peerID)
			if err == context.Canceled {
				return
			}
			log.Error(err)
		}
		h, err := s.processResponse(r)
		if err != nil {
			log.Error(err)
			// logFail and move to the next attempt
			s.peerTracker.logFail(peerID)
			continue
		}
		// if there were no errors - logSuccess and send headers to the result channel
		s.peerTracker.logSuccess(peerID, size, duration)
		headers <- h
		return
	}
	s.cancel()
	s.errCh <- errors.New("max attempt reached")
}

// GetRangeByHeight requests headers from different peers.
func (s *session) GetRangeByHeight(ctx context.Context, from, amount uint64) chan []*header.ExtendedHeader {
	log.Debugw("requesting headers", "from", from, "to", from+amount)
	requests := prepareRequests(from, amount, chunkSize)
	result := make(chan []*header.ExtendedHeader, len(requests))
	defer func() {
		if len(requests) == 0 {
			close(result)
		}
	}()
	for _, req := range requests {
		go s.doRequest(ctx, req, result)
	}

	return result
}

// requestHeaders sends the ExtendedHeaderRequest to a remote peer.
func (s *session) requestHeaders(
	ctx context.Context,
	to peer.ID,
	req *p2p_pb.ExtendedHeaderRequest,
) ([]*p2p_pb.ExtendedHeaderResponse, int64, int64, error) {
	stream, err := s.host.NewStream(ctx, to, exchangeProtocolID)
	if err != nil {
		return nil, 0, 0, err
	}
	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Debugf("error setting deadline: %s", err)
	}
	now := time.Now()
	// send request
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, 0, 0, err
	}
	err = stream.CloseWrite()
	if err != nil {
		log.Error(err)
	}
	responses := make([]*p2p_pb.ExtendedHeaderResponse, 0)
	size := int64(0)
	for i := 0; i < int(req.Amount); i++ {
		resp := new(p2p_pb.ExtendedHeaderResponse)
		if err = stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			log.Debugf("error setting deadline: %s", err)
		}
		_, err := serde.Read(stream, resp)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, 0, 0, err
		}
		size += int64(unsafe.Sizeof(*resp))
		responses = append(responses, resp)
	}
	duration := time.Since(now).Milliseconds()
	if err = stream.Close(); err != nil {
		log.Errorw("closing stream", "err", err)
	}
	return responses, size, duration, nil
}

// processResponse converts ExtendedHeaderResponse to ExtendedHeader.
func (s *session) processResponse(responses []*p2p_pb.ExtendedHeaderResponse) ([]*header.ExtendedHeader, error) {
	headers := make([]*header.ExtendedHeader, 0)
	for _, resp := range responses {
		err := convertStatusCodeToError(resp.StatusCode)
		if err != nil {
			return nil, err
		}
		header, err := header.UnmarshalExtendedHeader(resp.Body)
		if err != nil {
			return nil, err
		}
		headers = append(headers, header)
	}

	return headers, nil
}

// prepareRequests converts incoming range into separate ExtendedHeaderRequest.
func prepareRequests(from, amount, chunkSize uint64) []*p2p_pb.ExtendedHeaderRequest {
	requests := make([]*p2p_pb.ExtendedHeaderRequest, 0)
	for amount >= uint64(0) {
		var requestSize uint64
		if amount < chunkSize {
			requestSize = amount
			amount = 0
		} else {
			amount -= chunkSize
			from += chunkSize
			requestSize = chunkSize
		}
		requests = append(requests, &p2p_pb.ExtendedHeaderRequest{
			Data:   &p2p_pb.ExtendedHeaderRequest_Origin{Origin: from},
			Amount: requestSize,
		})
	}
	return requests
}
