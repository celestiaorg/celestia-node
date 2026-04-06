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

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

var log = logging.Logger("rpc")

// TODO(@Wondertan): Expose in config if requested
const (
	// maxRequestSize is 5 MiB, significantly lower than go-jsonrpc's 100 MiB default.
	maxRequestSize = 5 << 20
	// maxConcurrentConns caps simultaneous connections to bound goroutine/FD usage.
	maxConcurrentConns = 500
	// maxRequestsPerSecond is the per-IP sustained rate.
	maxRequestsPerSecond = 100
	// maxRequestBurst is the per-IP burst allowance.
	maxRequestBurst = 200
)

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

	started    atomic.Bool
	cancelFunc context.CancelFunc
	corsConfig CORSConfig

	tlsEnabled  bool
	tlsCertPath string
	tlsKeyPath  string

	signer   jwt.Signer
	verifier jwt.Verifier
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
	signer jwt.Signer,
	verifier jwt.Verifier,
) *Server {
	rpc := jsonrpc.NewServer(
		jsonrpc.WithMaxRequestSize(maxRequestSize),
	)

	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		rpc:          rpc,
		signer:       signer,
		verifier:     verifier,
		authDisabled: authDisabled,
		cancelFunc:   cancel,
		corsConfig:   corsConfig,
		tlsEnabled:   tlsConfig.Enabled,
		tlsCertPath:  tlsConfig.CertPath,
		tlsKeyPath:   tlsConfig.KeyPath,
	}

	srv.srv = &http.Server{
		Addr:    net.JoinHostPort(address, port),
		Handler: srv.newHandlerStack(ctx, rpc),
		// the amount of time allowed to read request headers. set to the default 2 seconds
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	return srv
}

// newHandlerStack returns wrapped rpc related handlers.
// Middleware order (outermost first): rate-limit → conn-limit → CORS/auth → RPC handler.
func (s *Server) newHandlerStack(ctx context.Context, core http.Handler) http.Handler {
	var h http.Handler
	switch {
	case s.authDisabled:
		log.Warn("auth disabled, allowing all origins, methods and headers for CORS")
		h = s.corsAny(core)
	case s.corsConfig.Enabled:
		h = s.corsWithConfig(s.authHandler(core))
	default:
		h = s.authHandler(core)
	}

	// Apply connection and rate limiting as outermost layers.
	h = connLimit(maxConcurrentConns, h)
	h = rateLimit(ctx, maxRequestsPerSecond, maxRequestBurst, h)
	return h
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
	s.cancelFunc()
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
