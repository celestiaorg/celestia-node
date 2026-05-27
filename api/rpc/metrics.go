package rpc

import (
	"context"
	"net"
	"net/http"
	"reflect"
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
	gaugeReg            metric.Registration
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
	m.gaugeReg, err = meter.RegisterCallback(m.observeGauges, m.connectionsOpenInst)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *rpcMetrics) observeGauges(_ context.Context, obs metric.Observer) error {
	obs.ObserveInt64(m.connectionsOpenInst, m.connectionsOpen.Load())
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
