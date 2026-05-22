package rpc

import (
	"context"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("rpc")

// rpcMetrics holds the JSON-RPC server instruments. HTTP-layer metrics are
// recorded by the `instrument` middleware (no method label, since the HTTP
// middleware can't see the parsed JSON-RPC method without buffering bodies).
// Method-keyed call counts are recorded by `traceMethod`, hooked into
// jsonrpc.WithTracer so go-jsonrpc supplies the already-parsed method name.
type rpcMetrics struct {
	// HTTP-layer instruments, no method label.
	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestSize     metric.Int64Histogram
	responseSize    metric.Int64Histogram

	// methodCalls is method-keyed (via go-jsonrpc Tracer hook) with a
	// boolean error attribute. No duration: the Tracer signature doesn't
	// carry it, see go-jsonrpc handler.Tracer.
	methodCalls metric.Int64Counter

	// websocketUpgrades counts StateHijacked transitions. net/http emits
	// StateHijacked once per upgrade and does not emit a matching close,
	// so this is intrinsically cumulative — a counter, not a gauge.
	websocketUpgrades metric.Int64Counter

	// active requests + open HTTP connections — atomic counters surfaced
	// via observable gauges. Single consistent pattern, one atomic load
	// per collection cycle, no per-request SDK calls.
	activeRequests      atomic.Int64
	connectionsOpen     atomic.Int64
	activeRequestsInst  metric.Int64ObservableGauge
	connectionsOpenInst metric.Int64ObservableGauge
	gaugeReg            metric.Registration
}

func newMetrics() (*rpcMetrics, error) {
	m := &rpcMetrics{}

	var err error
	m.requestsTotal, err = meter.Int64Counter(
		"rpc_requests_total",
		metric.WithDescription("total number of JSON-RPC requests served, labeled by HTTP status"),
	)
	if err != nil {
		return nil, err
	}

	m.requestDuration, err = meter.Float64Histogram(
		"rpc_request_duration_seconds",
		metric.WithDescription("duration of JSON-RPC request handling"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	m.responseSize, err = meter.Int64Histogram(
		"rpc_response_size_bytes",
		metric.WithDescription("size of JSON-RPC response payload"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	m.requestSize, err = meter.Int64Histogram(
		"rpc_request_size_bytes",
		metric.WithDescription(
			"size of JSON-RPC request payload; only recorded when Content-Length>0 "+
				"(chunked transfer and empty-body GETs are skipped to keep quantiles meaningful)",
		),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

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

	m.activeRequestsInst, err = meter.Int64ObservableGauge(
		"rpc_active_requests",
		metric.WithDescription("number of JSON-RPC requests currently being served"),
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
	m.gaugeReg, err = meter.RegisterCallback(
		m.observeGauges,
		m.activeRequestsInst,
		m.connectionsOpenInst,
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *rpcMetrics) observeGauges(_ context.Context, obs metric.Observer) error {
	obs.ObserveInt64(m.activeRequestsInst, m.activeRequests.Load())
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

// instrument wraps the next handler with HTTP-layer RPC metrics. The HTTP
// middleware has no visibility into the JSON-RPC method (extracting it would
// require buffering the entire request body — see traceMethod for the
// method-keyed signal sourced from go-jsonrpc instead).
func (m *rpcMetrics) instrument(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		m.activeRequests.Add(1)
		defer m.activeRequests.Add(-1)

		// Only record when Content-Length is a real size. -1 means chunked
		// (unknown length); 0 means no body. Both would distort quantiles.
		if r.ContentLength > 0 {
			m.requestSize.Record(r.Context(), r.ContentLength)
		}

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		statusAttr := attribute.Int("status", rec.status)
		m.requestsTotal.Add(r.Context(), 1, metric.WithAttributes(statusAttr))
		m.requestDuration.Record(r.Context(), time.Since(start).Seconds())
		if rec.bytesWritten > 0 {
			m.responseSize.Record(r.Context(), rec.bytesWritten)
		}
	})
}

// responseRecorder captures status code and bytes written for metrics.
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
	wroteHeader  bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}
