package rpc

import (
	"context"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("rpc")

// rpcMetrics holds the JSON-RPC server instruments that live outside otelhttp's
// scope. HTTP request-level metrics (duration, sizes, active requests, status)
// are recorded by otelhttp wrapping the handler (see Server.newHandlerStack);
// here we keep only what otelhttp can't see:
//   - method-keyed call counts, via go-jsonrpc's Tracer hook (traceMethod),
//   - connection-lifecycle signals from http.Server.ConnState (onConnState).
type rpcMetrics struct {
	// methodCalls is method-keyed (via go-jsonrpc Tracer hook) with a boolean
	// error attribute. No duration: the Tracer signature doesn't carry it, see
	// go-jsonrpc handler.Tracer.
	methodCalls metric.Int64Counter

	// websocketUpgrades counts StateHijacked transitions. net/http emits
	// StateHijacked once per upgrade and does not emit a matching close, so this
	// is intrinsically cumulative — a counter, not a gauge.
	websocketUpgrades metric.Int64Counter

	// connectionsOpen tracks currently-open HTTP connections, surfaced via an
	// observable gauge. Connection lifecycle is invisible to otelhttp (it only
	// sees individual requests), so it stays instrumented here.
	connectionsOpen     atomic.Int64
	connectionsOpenInst metric.Int64ObservableGauge

	// websocketConnsOpen tracks currently-open websocket connections. It can't
	// live in onConnState like connectionsOpen: net/http emits StateHijacked on
	// upgrade but never a matching close for hijacked conns, so the gauge is
	// bracketed around the (blocking) ws ServeHTTP in trackWebsocket instead.
	// Live slot usage against maxConcurrentConns is connectionsOpen +
	// websocketConnsOpen (hijacked conns leave connectionsOpen on upgrade).
	websocketConnsOpen     atomic.Int64
	websocketConnsOpenInst metric.Int64ObservableGauge

	gaugeReg metric.Registration
}

func newMetrics() (*rpcMetrics, error) {
	m := &rpcMetrics{}

	var err error
	m.methodCalls, err = meter.Int64Counter(
		"rpc_method_calls_total",
		metric.WithDescription(
			"JSON-RPC method invocations, labeled by `method` and `error` "+
				"(true if the handler returned an error). Sourced from go-jsonrpc's Tracer hook.",
		),
	)
	if err != nil {
		return nil, err
	}

	m.websocketUpgrades, err = meter.Int64Counter(
		"rpc_websocket_upgrades_total",
		metric.WithDescription("HTTP connections hijacked by websocket upgrades"),
	)
	if err != nil {
		return nil, err
	}

	m.connectionsOpenInst, err = meter.Int64ObservableGauge(
		"rpc_connections_open",
		metric.WithDescription("number of HTTP connections currently open to the RPC server"),
	)
	if err != nil {
		return nil, err
	}
	m.websocketConnsOpenInst, err = meter.Int64ObservableGauge(
		"rpc_websocket_connections_open",
		metric.WithDescription("number of websocket connections currently open to the RPC server"),
	)
	if err != nil {
		return nil, err
	}

	m.gaugeReg, err = meter.RegisterCallback(
		m.observeGauges, m.connectionsOpenInst, m.websocketConnsOpenInst,
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *rpcMetrics) observeGauges(_ context.Context, obs metric.Observer) error {
	obs.ObserveInt64(m.connectionsOpenInst, m.connectionsOpen.Load())
	obs.ObserveInt64(m.websocketConnsOpenInst, m.websocketConnsOpen.Load())
	return nil
}

// onConnState is intended to be installed as http.Server.ConnState. Connection
// lifecycle: New → Active/Idle → (Closed | Hijacked). open = New − (Closed + Hijacked).
// StateHijacked also bumps the cumulative websocket-upgrade counter, since
// net/http never emits a matching close for hijacked connections.
func (m *rpcMetrics) onConnState(_ net.Conn, state http.ConnState) {
	if m == nil {
		return
	}
	switch state {
	case http.StateNew:
		m.connectionsOpen.Add(1)
	case http.StateHijacked:
		m.connectionsOpen.Add(-1)
		m.websocketUpgrades.Add(context.Background(), 1)
	case http.StateClosed:
		m.connectionsOpen.Add(-1)
	}
}

// traceMethod is installed via jsonrpc.WithTracer. go-jsonrpc invokes it after
// every RPC dispatch with the already-parsed method name and the handler's
// error (nil on success). No body sniffing, no extra JSON parsing on the
// hot path.
func (m *rpcMetrics) traceMethod(method string, _, _ []reflect.Value, err error) {
	if m == nil {
		return
	}
	m.methodCalls.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
		attribute.Bool("error", err != nil),
	))
}

// isWebsocketUpgrade reports whether r is a websocket handshake. Such a request
// is served by a single ServeHTTP call that blocks for the whole connection
// lifetime, so bracketing it yields an accurate live count — unlike ConnState,
// which sees StateHijacked but never a matching close.
func isWebsocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// trackWebsocket brackets each websocket connection with the websocketConnsOpen
// gauge. The inner ServeHTTP blocks until the socket closes, so the deferred
// decrement fires exactly on close — the close signal net/http withholds for
// hijacked connections. A handshake that fails to upgrade returns immediately,
// leaving the gauge net-zero.
func (m *rpcMetrics) trackWebsocket(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || !isWebsocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}
		m.websocketConnsOpen.Add(1)
		defer m.websocketConnsOpen.Add(-1)
		next.ServeHTTP(w, r)
	})
}
