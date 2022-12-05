package p2p

import (
	"context"
	"io"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/celestia-node/share/eds"
	p2p_pb "github.com/celestiaorg/celestia-node/share/eds/p2p/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
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
	// TODO(@distractedm1nd): set read deadline
	req := new(p2p_pb.EDSRequest)
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
	if err != nil {
		// TODO(@distractedm1nd): handle INVALID, REFUSED status codes
		// TODO(@distractedm1nd): write deadline
		resp := &p2p_pb.EDSResponse{Status: p2p_pb.Status_NOT_FOUND}
		_, err := serde.Write(stream, resp)
		if err != nil {
			log.Errorw("server: writing response to stream", "err", err, "dataHash", dataHash)
			stream.Reset() //nolint:errcheck
			return
		}
	}

	// TODO(@distractedm1nd): write deadline
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
