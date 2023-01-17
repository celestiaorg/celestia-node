package p2p

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
)

var (
	tracer = otel.Tracer("header/server")
)

// ExchangeServer represents the server-side component for
// responding to inbound header-related requests.
type ExchangeServer struct {
	protocolID protocol.ID

	host   host.Host
	getter header.Getter

	ctx    context.Context
	cancel context.CancelFunc

	Params ServerParameters
}

// NewExchangeServer returns a new P2P server that handles inbound
// header-related requests.
func NewExchangeServer(
	host host.Host,
	getter header.Getter,
	protocolSuffix string,
	opts ...Option[ServerParameters],
) (*ExchangeServer, error) {
	params := DefaultServerParameters()
	for _, opt := range opts {
		opt(&params)
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &ExchangeServer{
		protocolID: protocolID(protocolSuffix),
		host:       host,
		getter:     getter,
		Params:     params,
	}, nil
}

// Start sets the stream handler for inbound header-related requests.
func (serv *ExchangeServer) Start(context.Context) error {
	serv.ctx, serv.cancel = context.WithCancel(context.Background())
	log.Info("server: listening for inbound header requests")

	serv.host.SetStreamHandler(serv.protocolID, serv.requestHandler)

	return nil
}

// Stop removes the stream handler for serving header-related requests.
func (serv *ExchangeServer) Stop(context.Context) error {
	log.Info("server: stopping server")
	serv.cancel()
	serv.host.RemoveStreamHandler(serv.protocolID)
	return nil
}

// requestHandler handles inbound ExtendedHeaderRequests.
func (serv *ExchangeServer) requestHandler(stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(serv.Params.ReadDeadline))
	if err != nil {
		log.Debugf("error setting deadline: %s", err)
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
		log.Error(err)
	}

	var headers []*header.ExtendedHeader
	// retrieve and write ExtendedHeaders
	switch pbreq.Data.(type) {
	case *p2p_pb.ExtendedHeaderRequest_Hash:
		headers, err = serv.handleRequestByHash(pbreq.GetHash())
	case *p2p_pb.ExtendedHeaderRequest_Origin:
		headers, err = serv.handleRequest(pbreq.GetOrigin(), pbreq.GetOrigin()+pbreq.Amount)
	default:
		log.Error("server: invalid data type received")
		stream.Reset() //nolint:errcheck
		return
	}
	var code p2p_pb.StatusCode
	switch err {
	case nil:
		code = p2p_pb.StatusCode_OK
	case header.ErrNotFound:
		code = p2p_pb.StatusCode_NOT_FOUND
	default:
		stream.Reset() //nolint:errcheck
		return
	}

	// reallocate headers with 1 nil ExtendedHeader if code is not StatusCode_OK
	if code != p2p_pb.StatusCode_OK {
		headers = make([]*header.ExtendedHeader, 1)
	}
	// write all headers to stream
	for _, h := range headers {
		if err := stream.SetWriteDeadline(time.Now().Add(serv.Params.ReadDeadline)); err != nil {
			log.Debugf("error setting deadline: %s", err)
		}
		var bin []byte
		// if header is not nil, then marshal it to []byte.
		// if header is nil, then error was received,so we will set empty []byte to proto.
		if h != nil {
			bin, err = h.MarshalBinary()
			if err != nil {
				log.Errorw("server: marshaling header to proto", "height", h.Height, "err", err)
				stream.Reset() //nolint:errcheck
				return
			}
		}
		_, err = serde.Write(stream, &p2p_pb.ExtendedHeaderResponse{Body: bin, StatusCode: code})
		if err != nil {
			log.Errorw("server: writing header to stream", "err", err)
			stream.Reset() //nolint:errcheck
			return
		}
	}

	err = stream.Close()
	if err != nil {
		log.Errorw("while closing inbound stream", "err", err)
	}
}

// handleRequestByHash returns the ExtendedHeader at the given hash
// if it exists.
func (serv *ExchangeServer) handleRequestByHash(hash []byte) ([]*header.ExtendedHeader, error) {
	log.Debugw("server: handling header request", "hash", tmbytes.HexBytes(hash).String())
	ctx, cancel := context.WithTimeout(serv.ctx, serv.Params.ServeTimeout)
	defer cancel()
	ctx, span := tracer.Start(ctx, "request-by-hash", trace.WithAttributes(
		attribute.String("hash", tmbytes.HexBytes(hash).String()),
	))
	defer span.End()

	h, err := serv.getter.Get(ctx, hash)
	if err != nil {
		log.Errorw("server: getting header by hash", "hash", tmbytes.HexBytes(hash).String(), "err", err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.AddEvent("fetched-header-from-store", trace.WithAttributes(
		attribute.String("hash", tmbytes.HexBytes(hash).String()),
		attribute.Int64("height", h.Height)),
	)
	span.SetStatus(codes.Ok, "")
	return []*header.ExtendedHeader{h}, nil
}

// handleRequest fetches the ExtendedHeader at the given origin and
// writes it to the stream.
func (serv *ExchangeServer) handleRequest(from, to uint64) ([]*header.ExtendedHeader, error) {
	if from == uint64(0) {
		return serv.handleHeadRequest()
	}

	ctx, cancel := context.WithTimeout(serv.ctx, serv.Params.ServeTimeout)
	defer cancel()

	ctx, span := tracer.Start(ctx, "request-range", trace.WithAttributes(
		attribute.Int64("from", int64(from)),
		attribute.Int64("to", int64(to))))
	defer span.End()

	if to-from > serv.Params.MaxRequestSize {
		log.Errorw("server: skip request for too many headers.", "amount", to-from)
		span.SetStatus(codes.Error, header.ErrHeadersLimitExceeded.Error())
		return nil, header.ErrHeadersLimitExceeded
	}

	log.Debugw("server: handling headers request", "from", from, "to", to)
	headersByRange, err := serv.getter.GetRangeByHeight(ctx, from, to)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warnw("server: requested headers not found", "from", from, "to", to)
			return nil, header.ErrNotFound
		}
		log.Errorw("server: getting headers", "from", from, "to", to, "err", err)
		return nil, err
	}

	span.AddEvent("fetched-range-of-headers", trace.WithAttributes(
		attribute.Int("amount", len(headersByRange))))
	span.SetStatus(codes.Ok, "")
	return headersByRange, nil
}

// handleHeadRequest returns the latest stored head.
func (serv *ExchangeServer) handleHeadRequest() ([]*header.ExtendedHeader, error) {
	log.Debug("server: handling head request")
	ctx, cancel := context.WithTimeout(serv.ctx, serv.Params.ServeTimeout)
	defer cancel()
	ctx, span := tracer.Start(ctx, "request-head")
	defer span.End()

	head, err := serv.getter.Head(ctx)
	if err != nil {
		log.Errorw("server: getting head", "err", err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.AddEvent("fetched-head", trace.WithAttributes(
		attribute.String("hash", head.Hash().String()),
		attribute.Int64("height", head.Height)),
	)
	span.SetStatus(codes.Ok, "")
	return []*header.ExtendedHeader{head}, nil
}
