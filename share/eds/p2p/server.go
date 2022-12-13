package p2p

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	p2p_pb "github.com/celestiaorg/celestia-node/share/eds/p2p/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

const (
	// writeDeadline sets timeout for sending messages to the stream
	writeDeadline = time.Second * 5
	// readDeadline sets timeout for reading messages from the stream
	readDeadline = time.Minute
)

// Server is responsible for serving ODSs for blocksync over the ShrEx/EDS protocol.
// This server is run by bridge nodes and full nodes. For more information, see ADR #13
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	store *eds.Store
}

// NewServer creates a new ShrEx/EDS server.
func NewServer(host host.Host, store *eds.Store, protocolSuffix string) *Server {
	return &Server{
		host:       host,
		store:      store,
		protocolID: protocolID(protocolSuffix),
	}
}

func (s *Server) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.host.SetStreamHandler(s.protocolID, s.handleStream)
	return nil
}

func (s *Server) Stop(context.Context) error {
	defer s.cancel()
	s.host.RemoveStreamHandler(s.protocolID)
	return nil
}

func (s *Server) handleStream(stream network.Stream) {
	log.Debug("server: handling eds request")

	// read request from stream to get the dataHash for store lookup
	req, err := s.readRequest(stream)
	if err != nil {
		log.Errorw("server: reading request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	// ensure the requested dataHash is a valid root
	err = share.DataHash(req.Hash).Validate()
	if err != nil {
		stream.Reset() //nolint:errcheck
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, readDeadline)
	defer cancel()
	status := p2p_pb.Status_OK
	// determine whether the EDS is available in our store
	edsReader, err := s.store.GetCAR(ctx, req.Hash)
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
		return
	}
}

func (s *Server) readRequest(stream network.Stream) (*p2p_pb.EDSRequest, error) {
	err := stream.SetReadDeadline(time.Now().Add(readDeadline))
	if err != nil {
		log.Warn(err)
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
	err := stream.SetWriteDeadline(time.Now().Add(writeDeadline))
	if err != nil {
		log.Warn(err)
	}

	resp := &p2p_pb.EDSResponse{Status: status}
	_, err = serde.Write(stream, resp)
	return err
}

func (s *Server) writeODS(edsReader io.ReadCloser, stream network.Stream) error {
	err := stream.SetWriteDeadline(time.Now().Add(writeDeadline))
	if err != nil {
		log.Warn(err)
	}

	odsReader, err := eds.ODSReader(edsReader)
	if err != nil {
		return fmt.Errorf("creating ODS reader: %w", err)
	}
	_, err = io.Copy(stream, odsReader)
	if err != nil {
		return fmt.Errorf("writing ODS bytes: %w", err)
	}

	return nil
}
