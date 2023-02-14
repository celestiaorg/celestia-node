package p2p

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/libs/header"
	p2p_pb "github.com/celestiaorg/celestia-node/libs/header/p2p/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

var (
	meter = global.MeterProvider().Meter("libs/header/p2p")
)

// ExchangeProxy is a wrapper around an Exchange[H] that records metrics for the Exchange[H].
type ExchangeProxy[H header.Header] struct {
	*Exchange[H]

	responseSize     syncfloat64.Histogram
	responseDuration syncfloat64.Histogram
}

// NewExchangeProxy creates a new ExchangeProxy[H] that wraps the given Exchange[H].
func NewExchangeProxy[H header.Header](ex *Exchange[H]) *ExchangeProxy[H] {
	log.Debug("Using Proxied P2P Exchange")
	responseSize, err := meter.
		SyncFloat64().
		Histogram(
			"get_headers_response_size",
			instrument.WithDescription("Size of get headers response in bytes"),
		)
	if err != nil {
		panic(err)
	}

	responseDuration, err := meter.
		SyncFloat64().
		Histogram(
			"get_headers_request_duration",
			instrument.WithDescription("Duration of get headers request in seconds"),
		)
	if err != nil {
		panic(err)
	}

	exp := &ExchangeProxy[H]{
		Exchange:         ex,
		responseSize:     responseSize,
		responseDuration: responseDuration,
	}
	ex.sendMsg = exp.sendMessage

	return exp
}

func (ex *ExchangeProxy[H]) Start(context.Context) error {
	log.Debug("Starting the proxied p2p exchange")
	return ex.Exchange.Start(context.Background())
}

// sendMessage is a wrapper around sendMessage that records metrics for the Exchange[H].
func (ex *ExchangeProxy[H]) sendMessage(
	ctx context.Context,
	host host.Host,
	to peer.ID,
	protocol protocol.ID,
	req *p2p_pb.HeaderRequest,
) ([]*p2p_pb.HeaderResponse, uint64, uint64, error) {
	log.Debug("wrapped sendMessage called")
	r, seize, duration, err := sendMessage(ctx, host, to, protocol, req)
	ex.responseSize.Record(ctx, float64(seize))
	ex.responseDuration.Record(ctx, float64(duration))
	return r, seize, duration, err
}

// newSession creates a new session for the given host.
// This method was created to enable proxying the session creation
// when enabling metrics.
func (ex *ExchangeProxy[H]) SessionFactory(
	ctx context.Context,
	h host.Host,
	peerTracker *peerTracker,
	protocolID protocol.ID,
	requestTimeout time.Duration,
	options ...option[H],
) *session[H] {
	log.Debug("proxied SessionFactory called")
	options = append(
		options,
		withSendMessageFunc[H](ex.sendMessage),
	)
	return newSession(ctx, h, peerTracker, protocolID, requestTimeout, options...)
}
