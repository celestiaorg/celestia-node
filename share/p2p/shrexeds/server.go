package shrexeds

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p"
	p2p_pb "github.com/celestiaorg/celestia-node/share/p2p/shrexeds/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// Server is responsible for serving ODSs for blocksync over the ShrEx/EDS protocol.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	store *eds.Store

	params *Parameters
}

// NewServer creates a new ShrEx/EDS server.
func NewServer(host host.Host, store *eds.Store, opts ...Option) (*Server, error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(params)
	}

	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-eds: server creation failed: %w", err)
	}

	return &Server{
		host:       host,
		store:      store,
		protocolID: protocolID(params.protocolSuffix),
		params:     params,
	}, nil
}

func (s *Server) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.host.SetStreamHandler(s.protocolID, p2p.RateLimitMiddleware(s.handleStream, s.params.concurrencyLimit))
	return nil
}

func (s *Server) Stop(context.Context) error {
	defer s.cancel()
	s.host.RemoveStreamHandler(s.protocolID)
	return nil
}

func (s *Server) handleStream(stream network.Stream) {
	log.Debug("server: handling eds request")

	// TODO: add read timeout!
	// read request from stream to get the dataHash for store lookup
	req, err := s.readRequest(stream)
	if err != nil {
		log.Errorw("server: reading request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	// ensure the requested dataHash is a valid root
	hash := share.DataHash(req.Hash)
	err = hash.Validate()
	if err != nil {
		stream.Reset() //nolint:errcheck
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, s.params.ReadCARDeadline)
	defer cancel()
	status := p2p_pb.Status_OK
	// determine whether the EDS is available in our store
	edsReader, err := s.store.GetCAR(ctx, hash)
	if err != nil {
		status = p2p_pb.Status_NOT_FOUND
	} else {
		defer edsReader.Close()
	}

	// inform the client of our status
	err = s.writeStatus(status, stream)
	if err != nil {
		log.Errorw("server: writing status to stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	// if we cannot serve the EDS, we are already done
	if status != p2p_pb.Status_OK {
		stream.Close()
		return
	}

	// start streaming the ODS to the client
	err = s.writeODS(edsReader, stream)
	if err != nil {
		log.Errorw("server: writing ods to stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("server: closing stream", "err", err)
	}
}

func (s *Server) readRequest(stream network.Stream) (*p2p_pb.EDSRequest, error) {
	err := stream.SetReadDeadline(time.Now().Add(s.params.ReadDeadline))
	if err != nil {
		log.Debug(err)
	}

	req := new(p2p_pb.EDSRequest)
	_, err = serde.Read(stream, req)
	if err != nil {
		return nil, err
	}
	err = stream.CloseRead()
	if err != nil {
		log.Error(err)
	}

	return req, nil
}

func (s *Server) writeStatus(status p2p_pb.Status, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(s.params.WriteDeadline))
	if err != nil {
		log.Debug(err)
	}

	resp := &p2p_pb.EDSResponse{Status: status}
	_, err = serde.Write(stream, resp)
	return err
}

func (s *Server) writeODS(edsReader io.ReadCloser, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(s.params.WriteDeadline))
	if err != nil {
		log.Debug(err)
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
