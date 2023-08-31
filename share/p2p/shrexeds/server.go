package shrexeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p"
	p2p_pb "github.com/celestiaorg/celestia-node/share/p2p/shrexeds/pb"
)

// Server is responsible for serving ODSs for blocksync over the ShrEx/EDS protocol.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	store *eds.Store

	params     *Parameters
	middleware *p2p.Middleware
	metrics    *p2p.Metrics
}

// NewServer creates a new ShrEx/EDS server.
func NewServer(params *Parameters, host host.Host, store *eds.Store) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: server creation failed: %w", err)
	}

	return &Server{
		host:       host,
		store:      store,
		protocolID: p2p.ProtocolID(params.NetworkID(), protocolString),
		params:     params,
		middleware: p2p.NewMiddleware(params.ConcurrencyLimit),
	}, nil
}

func (s *Server) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.host.SetStreamHandler(s.protocolID, s.middleware.RateLimitHandler(s.handleStream))
	return nil
}

func (s *Server) Stop(context.Context) error {
	defer s.cancel()
	s.host.RemoveStreamHandler(s.protocolID)
	return nil
}

func (s *Server) observeRateLimitedRequests() {
	numRateLimited := s.middleware.DrainCounter()
	if numRateLimited > 0 {
		s.metrics.ObserveRequests(context.Background(), numRateLimited, p2p.StatusRateLimited)
	}
}

func (s *Server) handleStream(stream network.Stream) {
	logger := log.With("peer", stream.Conn().RemotePeer().String())
	logger.Debug("server: handling eds request")

	s.observeRateLimitedRequests()

	// read request from stream to get the dataHash for store lookup
	req, err := s.readRequest(logger, stream)
	if err != nil {
		logger.Warnw("server: reading request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	// ensure the requested dataHash is a valid root
	hash := share.DataHash(req.Hash)
	err = hash.Validate()
	if err != nil {
		logger.Warnw("server: invalid request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	logger = logger.With("hash", hash.String())

	ctx, cancel := context.WithTimeout(s.ctx, s.params.HandleRequestTimeout)
	defer cancel()

	// determine whether the EDS is available in our store
	// we do not close the reader, so that other requests will not need to re-open the file.
	// closing is handled by the LRU cache.
	edsReader, err := s.store.GetCAR(ctx, hash)
	var status p2p_pb.Status
	switch {
	case err == nil:
		defer func() {
			if err := edsReader.Close(); err != nil {
				log.Warnw("closing car reader", "err", err)
			}
		}()
		edsReader.Close()
		status = p2p_pb.Status_OK
	case errors.Is(err, eds.ErrNotFound):
		logger.Warnw("server: request hash not found")
		s.metrics.ObserveRequests(ctx, 1, p2p.StatusNotFound)
		status = p2p_pb.Status_NOT_FOUND
	case err != nil:
		logger.Errorw("server: get CAR", "err", err)
		status = p2p_pb.Status_INTERNAL
	}

	// inform the client of our status
	err = s.writeStatus(logger, status, stream)
	if err != nil {
		logger.Warnw("server: writing status to stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	// if we cannot serve the EDS, we are already done
	if status != p2p_pb.Status_OK {
		err = stream.Close()
		if err != nil {
			logger.Debugw("server: closing stream", "err", err)
		}
		return
	}

	// start streaming the ODS to the client
	err = s.writeODS(logger, edsReader, stream)
	if err != nil {
		logger.Warnw("server: writing ods to stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	s.metrics.ObserveRequests(ctx, 1, p2p.StatusSuccess)
	err = stream.Close()
	if err != nil {
		logger.Debugw("server: closing stream", "err", err)
	}
}

func (s *Server) readRequest(logger *zap.SugaredLogger, stream network.Stream) (*p2p_pb.EDSRequest, error) {
	err := stream.SetReadDeadline(time.Now().Add(s.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("server: set read deadline", "err", err)
	}

	req := new(p2p_pb.EDSRequest)
	_, err = serde.Read(stream, req)
	if err != nil {
		return nil, err
	}
	err = stream.CloseRead()
	if err != nil {
		logger.Debugw("server: closing read", "err", err)
	}

	return req, nil
}

func (s *Server) writeStatus(logger *zap.SugaredLogger, status p2p_pb.Status, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(s.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("server: set write deadline", "err", err)
	}

	resp := &p2p_pb.EDSResponse{Status: status}
	_, err = serde.Write(stream, resp)
	return err
}

func (s *Server) writeODS(logger *zap.SugaredLogger, edsReader io.Reader, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(s.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("server: set read deadline", "err", err)
	}

	odsReader, err := eds.ODSReader(edsReader)
	if err != nil {
		return fmt.Errorf("creating ODS reader: %w", err)
	}
	buf := make([]byte, s.params.BufferSize)
	_, err = io.CopyBuffer(stream, odsReader, buf)
	if err != nil {
		return fmt.Errorf("writing ODS bytes: %w", err)
	}

	return nil
}
