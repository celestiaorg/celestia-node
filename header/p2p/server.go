package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
)

// ExchangeServer represents the server-side component for
// responding to inbound header-related requests.
type ExchangeServer struct {
	host  host.Host
	store header.Store

	ctx    context.Context
	cancel context.CancelFunc
}

// NewExchangeServer returns a new P2P server that handles inbound
// header-related requests.
func NewExchangeServer(host host.Host, store header.Store) *ExchangeServer {
	return &ExchangeServer{
		host:  host,
		store: store,
	}
}

// Start sets the stream handler for inbound header-related requests.
func (serv *ExchangeServer) Start(context.Context) error {
	serv.ctx, serv.cancel = context.WithCancel(context.Background())
	log.Info("server: listening for inbound header requests")

	serv.host.SetStreamHandler(exchangeProtocolID, serv.requestHandler)

	return nil
}

// Stop removes the stream handler for serving header-related requests.
func (serv *ExchangeServer) Stop(context.Context) error {
	log.Info("server: stopping server")
	serv.cancel()
	serv.host.RemoveStreamHandler(exchangeProtocolID)
	return nil
}

// requestHandler handles inbound ExtendedHeaderRequests.
func (serv *ExchangeServer) requestHandler(stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(readDeadline))
	if err != nil {
		log.Warn(err)
	}
	// unmarshal request
	pbreq := new(p2p_pb.ExtendedHeaderRequest)
	_, err = serde.Read(stream, pbreq)
	if err != nil {
		log.Errorw("server: reading header request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	if err = stream.CloseRead(); err != nil {
		log.Warn(err)
	}
	// retrieve and write ExtendedHeaders
	switch pbreq.Data.(type) {
	case *p2p_pb.ExtendedHeaderRequest_Hash:
		serv.handleRequestByHash(pbreq.GetHash(), stream)
	case *p2p_pb.ExtendedHeaderRequest_Origin:
		serv.handleRequest(pbreq.GetOrigin(), pbreq.GetOrigin()+pbreq.Amount, stream)
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("while closing inbound stream", "err", err)
	}
}

// handleRequestByHash returns the ExtendedHeader at the given hash
// if it exists.
func (serv *ExchangeServer) handleRequestByHash(hash []byte, stream network.Stream) {
	log.Debugw("server: handling header request", "hash", tmbytes.HexBytes(hash).String())

	h, err := serv.store.Get(serv.ctx, hash)
	if err != nil {
		log.Errorw("server: getting header by hash", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	resp, err := header.ExtendedHeaderToProto(h)
	if err != nil {
		log.Errorw("server: marshaling header to proto", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorw("server: writing header to stream", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
}

// handleRequest fetches the ExtendedHeader at the given origin and
// writes it to the stream.
func (serv *ExchangeServer) handleRequest(from, to uint64, stream network.Stream) {
	var headers []*header.ExtendedHeader
	if from == uint64(0) {
		log.Debug("server: handling head request")

		head, err := serv.store.Head(serv.ctx)
		if err != nil {
			log.Errorw("server: getting head", "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
		headers = make([]*header.ExtendedHeader, 1)
		headers[0] = head
	} else {
		log.Debugw("server: handling headers request", "from", from, "to", to)

		headersByRange, err := serv.store.GetRangeByHeight(serv.ctx, from, to)
		if err != nil {
			log.Errorw("server: getting headers", "from", from, "to", to, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
		headers = headersByRange
	}

	if err := stream.SetWriteDeadline(time.Now().Add(writeDeadline * time.Duration(len(headers)))); err != nil {
		log.Warn(err)
	}
	// write all headers to stream
	for _, h := range headers {
		resp, err := header.ExtendedHeaderToProto(h)
		if err != nil {
			log.Errorw("server: marshaling header to proto", "height", h.Height, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}

		_, err = serde.Write(stream, resp)
		if err != nil {
			log.Errorw("server: writing header to stream", "height", h.Height, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
	}
}
