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

	"github.com/celestiaorg/celestia-node/libs/utils"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/celestia-node/store"
)

// Server is responsible for serving ODSs for blocksync over the ShrEx/EDS protocol.
type Server struct {
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	store *store.Store

	params     *Parameters
	middleware *shrex.Middleware
	metrics    *shrex.Metrics
}

// NewServer creates a new ShrEx/EDS server.
func NewServer(params *Parameters, host host.Host, store *store.Store) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: server creation failed: %w", err)
	}

	return &Server{
		host:       host,
		store:      store,
		protocolID: shrex.ProtocolID(params.NetworkID(), protocolString),
		params:     params,
		middleware: shrex.NewMiddleware(params.ConcurrencyLimit),
	}, nil
}

func (s *Server) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.host.SetStreamHandler(s.protocolID, s.middleware.RateLimitHandler(s.streamHandler(ctx)))
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
		s.metrics.ObserveRequests(context.Background(), numRateLimited, shrex.StatusRateLimited)
	}
}

func (s *Server) streamHandler(ctx context.Context) network.StreamHandler {
	return func(stream network.Stream) {
		err := s.handleEDS(ctx, stream)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return
		}
		s.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
		if err = stream.Close(); err != nil {
			log.Debugw("server: closing stream", "err", err)
		}
	}
}

func (s *Server) handleEDS(ctx context.Context, stream network.Stream) error {
	logger := log.With("peer", stream.Conn().RemotePeer().String())
	logger.Debug("server: handling eds request")
	s.observeRateLimitedRequests()

	// read request from stream to get the dataHash for store lookup
	id, err := s.readRequest(logger, stream)
	if err != nil {
		logger.Warnw("server: reading request from stream", "err", err)
		return err
	}

	logger = logger.With("height", id.Height)

	ctx, cancel := context.WithTimeout(ctx, s.params.HandleRequestTimeout)
	defer cancel()

	// determine whether the EDS is available in our store
	// we do not close the reader, so that other requests will not need to re-open the file.
	// closing is handled by the LRU cache.
	file, err := s.store.GetByHeight(ctx, id.Height)
	var status shrexpb.Status
	switch {
	case err == nil:
		defer utils.CloseAndLog(logger, "file", file)
		status = shrexpb.Status_OK
	case errors.Is(err, store.ErrNotFound):
		logger.Warnw("server: request height not found")
		s.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
		status = shrexpb.Status_NOT_FOUND
	case err != nil:
		logger.Errorw("server: get file", "err", err)
		status = shrexpb.Status_INTERNAL
	}

	// inform the client of our status
	err = s.writeStatus(logger, status, stream)
	if err != nil {
		logger.Warnw("server: writing status to stream", "err", err)
		return err
	}
	// if we cannot serve the EDS, we are already done
	if status != shrexpb.Status_OK {
		return nil
	}

	// start streaming the ODS to the client
	err = s.writeODS(logger, file, stream)
	if err != nil {
		logger.Warnw("server: writing ods to stream", "err", err)
		return err
	}
	return nil
}

func (s *Server) readRequest(logger *zap.SugaredLogger, stream network.Stream) (shwap.EdsID, error) {
	err := stream.SetReadDeadline(time.Now().Add(s.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("server: set read deadline", "err", err)
	}

	edsIDs := shwap.EdsID{}
	_, err = edsIDs.ReadFrom(stream)
	if err != nil {
		return shwap.EdsID{}, fmt.Errorf("reading request: %w", err)
	}
	err = stream.CloseRead()
	if err != nil {
		logger.Warnw("server: closing read", "err", err)
	}
	return edsIDs, nil
}

func (s *Server) writeStatus(logger *zap.SugaredLogger, status shrexpb.Status, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(s.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("server: set write deadline", "err", err)
	}

	resp := &shrexpb.Response{Status: status}
	_, err = serde.Write(stream, resp)
	return err
}

func (s *Server) writeODS(logger *zap.SugaredLogger, streamer eds.Streamer, stream network.Stream) error {
	reader, err := streamer.Reader()
	if err != nil {
		return fmt.Errorf("getting ODS reader: %w", err)
	}
	err = stream.SetWriteDeadline(time.Now().Add(s.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("server: set read deadline", "err", err)
	}

	buf := make([]byte, s.params.BufferSize)
	n, err := io.CopyBuffer(stream, reader, buf)
	if err != nil {
		return fmt.Errorf("written: %v, writing ODS bytes: %w", n, err)
	}

	logger.Debugw("server: wrote ODS", "bytes", n)
	return nil
}
