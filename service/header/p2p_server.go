package header

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	pb "github.com/celestiaorg/celestia-node/service/header/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// P2PExchangeServer represents the server-side component for
// responding to inbound header-related requests.
type P2PExchangeServer struct {
	host  host.Host
	store Store

	ctx    context.Context
	cancel context.CancelFunc
}

// NewP2PExchangeServer returns a new P2P server that handles inbound
// header-related requests.
func NewP2PExchangeServer(host host.Host, store Store) *P2PExchangeServer {
	return &P2PExchangeServer{
		host:  host,
		store: store,
	}
}

// Start sets the stream handler for inbound header-related requests.
func (serv *P2PExchangeServer) Start(context.Context) error {
	serv.ctx, serv.cancel = context.WithCancel(context.Background())
	log.Info("p2p-server: listening for inbound header requests")

	serv.host.SetStreamHandler(exchangeProtocolID, serv.requestHandler)

	return nil
}

// Stop removes the stream handler for serving header-related requests.
func (serv *P2PExchangeServer) Stop(context.Context) error {
	log.Info("p2p-server: stopping server")
	serv.cancel()
	serv.host.RemoveStreamHandler(exchangeProtocolID)
	return nil
}

// requestHandler handles inbound ExtendedHeaderRequests.
func (serv *P2PExchangeServer) requestHandler(stream network.Stream) {
	// unmarshal request
	pbreq := new(pb.ExtendedHeaderRequest)
	_, err := serde.Read(stream, pbreq)
	if err != nil {
		log.Errorw("p2p-server: reading header request from stream", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	// retrieve and write ExtendedHeaders
	if pbreq.Hash != nil {
		serv.handleRequestByHash(pbreq.Hash, stream)
	} else {
		serv.handleRequest(pbreq.Origin, pbreq.Origin+pbreq.Amount, stream)
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("while closing inbound stream", "err", err)
	}
}

// handleRequestByHash returns the ExtendedHeader at the given hash
// if it exists.
func (serv *P2PExchangeServer) handleRequestByHash(hash []byte, stream network.Stream) {
	log.Debugw("p2p-server: handling header request", "hash", tmbytes.HexBytes(hash).String())

	header, err := serv.store.Get(serv.ctx, hash)
	if err != nil {
		log.Errorw("p2p-server: getting header by hash", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	resp, err := ExtendedHeaderToProto(header)
	if err != nil {
		log.Errorw("p2p-server: marshaling header to proto", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorw("p2p-server: writing header to stream", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
}

// handleRequest fetches the ExtendedHeader at the given origin and
// writes it to the stream.
func (serv *P2PExchangeServer) handleRequest(from, to uint64, stream network.Stream) {
	var headers []*ExtendedHeader
	if from == uint64(0) {
		log.Debug("p2p-server: handling head request")

		head, err := serv.store.Head(serv.ctx)
		if err != nil {
			log.Errorw("p2p-server: getting head", "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
		headers = make([]*ExtendedHeader, 1)
		headers[0] = head
	} else {
		log.Debugw("p2p-server: handling headers request", "from", from, "to", to)

		headersByRange, err := serv.store.GetRangeByHeight(serv.ctx, from, to)
		if err != nil {
			log.Errorw("p2p-server: getting headers", "from", from, "to", to, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
		headers = headersByRange
	}
	// write all headers to stream
	for _, header := range headers {
		resp, err := ExtendedHeaderToProto(header)
		if err != nil {
			log.Errorw("p2p-server: marshaling header to proto", "height", header.Height, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}

		_, err = serde.Write(stream, resp)
		if err != nil {
			log.Errorw("p2p-server: writing header to stream", "height", header.Height, "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
	}
}
