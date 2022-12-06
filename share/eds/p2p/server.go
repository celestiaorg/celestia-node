package p2p

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

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

// Server is responsible for serving EDSs for blocksync over the ShrEx/EDS protocol.
// This client is run by bridge nodes and full nodes. For more information, see ADR #13
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	store *eds.Store
}

// NewServer creates a new ShrEx/EDS server.
func NewServer(host host.Host, store *eds.Store) *Server {
	return &Server{
		host:       host,
		protocolID: protocolID,
		store:      store,
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
		return
	}

	// determine whether the EDS is available in our store
	edsReader, err := s.store.GetCARTemp(s.ctx, req.Hash)
	// TODO(@distractedm1nd): handle INVALID, REFUSED status codes
	var status p2p_pb.Status
	if err != nil {
		status = p2p_pb.Status_NOT_FOUND
	} else {
		status = p2p_pb.Status_OK
	}

	// inform the client of our status
	if err = s.writeStatus(status, stream); err != nil {
		log.Errorw("server: writing status to stream", "err", err)
		return
	}
	// if we cannot serve the EDS, we are done
	if status != p2p_pb.Status_OK {
		stream.Close()
		return
	}

	// start streaming the ODS to the client
	if err = s.writeODS(edsReader, stream); err != nil {
		log.Errorw("server: writing ods to stream", "err", err)
		return
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("server: closing stream", "err", err)
		return
	}
}

func (s *Server) readRequest(stream network.Stream) (*p2p_pb.EDSRequest, error) {
	if err := stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		log.Warn(err)
	}
	req := new(p2p_pb.EDSRequest)
	_, err := serde.Read(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	if err = stream.CloseRead(); err != nil {
		log.Error(err)
	}

	return req, err
}

func (s *Server) writeStatus(status p2p_pb.Status, stream network.Stream) error {
	if err := stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	resp := &p2p_pb.EDSResponse{Status: status}
	_, err := serde.Write(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return err
	}

	return nil
}

func (s *Server) writeODS(edsReader io.ReadCloser, stream network.Stream) error {
	if err := stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	odsReader, err := eds.ODSReader(edsReader)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return fmt.Errorf("creating ODS reader: %w", err)
	}
	odsBytes, err := io.ReadAll(odsReader)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return fmt.Errorf("reading ODS bytes: %w", err)
	}
	_, err = stream.Write(odsBytes)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return fmt.Errorf("writing ODS bytes: %w", err)
	}

	return nil
}
