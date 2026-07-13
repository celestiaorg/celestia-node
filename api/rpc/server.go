package rpc

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/celestiaorg/celestia-app/v9/pkg/appconsts"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

var log = logging.Logger("rpc")

const (
	// maxRequestSize bounds JSON-RPC request bodies. Sized as 2× appconsts.MaxTxSize
	// to cover base64 + JSON envelope overhead (~1.5×) on a worst-case blob.Submit,
	// which packs all blobs into a single PFB tx capped at MaxTxSize.
	maxRequestSize = int64(appconsts.MaxTxSize * 2)
	// maxConcurrentConns caps simultaneous connections to bound goroutine/FD usage.
	maxConcurrentConns = 500
)

// RateLimitConfig configures per-IP rate limiting based on the connection's
// remote address. When the node runs behind a reverse proxy, RemoteAddr is
// the proxy's address, so all clients share one bucket — in that case keep
// this disabled and apply rate limiting at the proxy instead.
type RateLimitConfig struct {
	// Enabled toggles the per-IP rate limit middleware.
	Enabled bool
	// RequestsPerSec is the sustained per-IP request rate; requests above
	// this rate (after burst is drained) get 429 Too Many Requests.
	RequestsPerSec int
	// Burst is the per-IP burst allowance — the bucket size that absorbs
	// short spikes before sustained rate kicks in.
	Burst int
	// CacheSize is the max number of per-IP buckets retained.
	// When exceeded, the least-recently-seen IP is evicted (and gets a fresh
	// burst on its next request). Bounds memory under unique-IP floods.
	CacheSize int
}

type CORSConfig struct {
	Enabled        bool
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

type Server struct {
	srv          *http.Server
	rpc          *jsonrpc.RPCServer
	listener     net.Listener
	authDisabled bool

	started      atomic.Bool
	corsConfig   CORSConfig
	rateLimitCfg RateLimitConfig

	tlsEnabled  bool
	tlsCertPath string
	tlsKeyPath  string

	signer   jwt.Signer
	verifier jwt.Verifier

	metrics *rpcMetrics
}

type TLSConfig struct {
	Enabled  bool
	CertPath string
	KeyPath  string
}

func NewServer(
	address, port string,
	authDisabled bool,
	corsConfig CORSConfig,
	tlsConfig TLSConfig,
	rateLimitCfg RateLimitConfig,
	signer jwt.Signer,
	verifier jwt.Verifier,
) *Server {
	srv := &Server{
		signer:       signer,
		verifier:     verifier,
		authDisabled: authDisabled,
		corsConfig:   corsConfig,
		rateLimitCfg: rateLimitCfg,
		tlsEnabled:   tlsConfig.Enabled,
		tlsCertPath:  tlsConfig.CertPath,
		tlsKeyPath:   tlsConfig.KeyPath,
	}

	// The tracer closure reads srv.metrics lazily: WithMetrics may run after
	// NewServer, and traceMethod is nil-safe. So we wire the hook now once
	// and let metrics get populated later.
	srv.rpc = jsonrpc.NewServer(
		jsonrpc.WithMaxRequestSize(maxRequestSize),
		jsonrpc.WithTracer(func(method string, params, results []reflect.Value, err error) {
			srv.metrics.traceMethod(method, params, results, err)
		}),
	)

	srv.srv = &http.Server{
		Addr:    net.JoinHostPort(address, port),
		Handler: srv.newHandlerStack(srv.rpc),
		// the amount of time allowed to read request headers. set to the default 2 seconds
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute, // 5m covers long unary calls
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	return srv
}

// newHandlerStack returns wrapped rpc related handlers.
// Middleware order (outermost first): rate-limit (opt-in) → conn-limit → CORS/auth → metrics → RPC handler.
func (s *Server) newHandlerStack(core http.Handler) http.Handler {
	// otelhttp records HTTP request-level metrics (duration, request/response
	// sizes, active requests, status) and — unlike a hand-rolled wrapper —
	// preserves the http.Hijacker that websocket upgrades rely on (it wraps the
	// writer via httpsnoop). Connection- and method-level signals live in
	// rpcMetrics. s.metrics may be nil (metrics disabled) → no wrapping.
	h := core
	if s.metrics != nil {
		// trackWebsocket wraps core directly so its bracket is the tightest
		// possible around the (blocking) ws ServeHTTP; otelhttp then wraps that.
		h = s.metrics.trackWebsocket(h)
		h = otelhttp.NewHandler(h, "rpc",
			// metrics only: suppress the per-request span otelhttp would emit.
			otelhttp.WithTracerProvider(noop.NewTracerProvider()),
		)
	}
	switch {
	case s.authDisabled:
		log.Warn("auth disabled, allowing all origins, methods and headers for CORS")
		h = s.corsAny(h)
	case s.corsConfig.Enabled:
		h = s.corsWithConfig(s.authHandler(h))
	default:
		h = s.authHandler(h)
	}

	h = connLimit(maxConcurrentConns, h)
	// Per-IP rate limiting is opt-in: behind a reverse proxy all clients share
	// one bucket (RemoteAddr == proxy), so the limit is best applied there.
	if s.rateLimitCfg.Enabled {
		h = rateLimit(s.rateLimitCfg.RequestsPerSec, s.rateLimitCfg.Burst, s.rateLimitCfg.CacheSize, h)
	}
	return h
}

// WithMetrics enables OTel metrics on the RPC server. Must be called before
// Start, since it rebuilds the handler stack to install the metrics middleware
// and registers an http.Server.ConnState hook for connection-level gauges.
func (s *Server) WithMetrics() error {
	m, err := newMetrics()
	if err != nil {
		return err
	}
	s.metrics = m
	s.srv.Handler = s.newHandlerStack(s.rpc)
	s.srv.ConnState = m.onConnState
	return nil
}

// verifyAuth is the RPC server's auth middleware. This middleware is only
// reached if a token is provided in the header of the request, otherwise only
// methods with `read` permissions are accessible.
func (s *Server) verifyAuth(_ context.Context, token string) ([]auth.Permission, error) {
	if s.authDisabled {
		return perms.AllPerms, nil
	}
	return authtoken.ExtractSignedPermissions(s.verifier, token)
}

// authHandler wraps the handler with authentication.
func (s *Server) authHandler(next http.Handler) http.Handler {
	return &auth.Handler{
		Verify: s.verifyAuth,
		Next:   next.ServeHTTP,
	}
}

// corsAny applies permissive CORS (allows all origins, methods, headers)
func (s *Server) corsAny(next http.Handler) http.Handler {
	c := cors.AllowAll()
	return c.Handler(next)
}

// corsWithConfig applies CORS with specific configuration
func (s *Server) corsWithConfig(next http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins: s.corsConfig.AllowedOrigins,
		AllowedMethods: s.corsConfig.AllowedMethods,
		AllowedHeaders: s.corsConfig.AllowedHeaders,
	})
	return c.Handler(next)
}

// RegisterService registers a service onto the RPC server. All methods on the service will then be
// exposed over the RPC.
func (s *Server) RegisterService(namespace string, service, out any) {
	if s.authDisabled {
		s.rpc.Register(namespace, service)
		return
	}

	auth.PermissionedProxy(perms.AllPerms, perms.DefaultPerms, service, getInternalStruct(out))
	s.rpc.Register(namespace, out)
}

func getInternalStruct(api any) any {
	return reflect.ValueOf(api).Elem().FieldByName("Internal").Addr().Interface()
}

// Start starts the RPC Server.
func (s *Server) Start(context.Context) error {
	couldStart := s.started.CompareAndSwap(false, true)
	if !couldStart {
		log.Warn("cannot start server: already started")
		return nil
	}
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		s.started.Store(false)
		return err
	}
	s.listener = listener
	if s.tlsEnabled {
		if _, err := tls.LoadX509KeyPair(s.tlsCertPath, s.tlsKeyPath); err != nil {
			s.started.Store(false)
			s.listener = nil
			listener.Close()
			return err
		}
		log.Infow("server started with TLS", "listening on", s.srv.Addr)
		//nolint:errcheck
		go s.srv.ServeTLS(listener, s.tlsCertPath, s.tlsKeyPath)
	} else {
		log.Infow("server started", "listening on", s.srv.Addr)
		//nolint:errcheck
		go s.srv.Serve(listener)
	}
	return nil
}

// Stop stops the RPC Server.
func (s *Server) Stop(ctx context.Context) error {
	couldStop := s.started.CompareAndSwap(true, false)
	if !couldStop {
		log.Warn("cannot stop server: already stopped")
		return nil
	}
	err := s.srv.Shutdown(ctx)
	if err != nil {
		return err
	}
	s.listener = nil
	log.Info("server stopped")
	return nil
}

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
