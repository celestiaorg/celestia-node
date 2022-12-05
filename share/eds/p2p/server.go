package p2p

import (
	"context"
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

type Server struct {
	host       host.Host
	protocolID protocol.ID

	store *eds.Store
}

func NewServer(host host.Host, store *eds.Store) *Server {
	return &Server{host: host, protocolID: protocolID, store: store}
}

func (s *Server) Start(context.Context) error {
	s.host.SetStreamHandler(s.protocolID, s.handleStream)
	return nil
}

func (s *Server) Stop(context.Context) error {
	s.host.RemoveStreamHandler(s.protocolID)
	return nil
}

func (s *Server) handleStream(stream network.Stream) {
	log.Debug("server: handling eds request")
	req := new(p2p_pb.EDSRequest)
	if err := stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		log.Warn(err)
	}
	_, err := serde.Read(stream, req)
	if err != nil {
		log.Errorw("server: reading header request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	if err = stream.CloseRead(); err != nil {
		log.Error(err)
	}

	ctx := context.Background()
	dataHash := req.Hash
	edsReader, err := s.store.GetCARTemp(ctx, dataHash)
	resp := &p2p_pb.EDSResponse{Status: p2p_pb.Status_OK}
	if err != nil {
		// TODO(@distractedm1nd): handle INVALID, REFUSED status codes
		resp = &p2p_pb.EDSResponse{Status: p2p_pb.Status_NOT_FOUND}
	}
	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorw("server: writing response to stream", "err", err, "dataHash", dataHash)
		stream.Reset() //nolint:errcheck
		return
	}

	// TODO(@distractedm1nd): question: do you set the write deadline for every single write?
	odsReader, err := eds.ODSReader(edsReader)
	if err != nil {
		log.Errorw("server: creating ODS reader", "err", err, "dataHash", dataHash)
		stream.Reset() //nolint:errcheck
		return
	}
	odsBytes, err := io.ReadAll(odsReader)
	if err != nil {
		log.Errorw("server: reading ODS bytes", "err", err, "dataHash", dataHash)
		stream.Reset() //nolint:errcheck
		return
	}
	// TODO(@distractedm1nd): we either need to send size first, or probably sending EOF after
	// writing the bytes if it doesn't already happen
	_, err = stream.Write(odsBytes)
	if err != nil {
		log.Errorw("server: writing ODS bytes", "err", err, "dataHash", dataHash)
		stream.Reset() //nolint:errcheck
		return
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("server: closing stream", "err", err)
		return
	}
}
