package gateway

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

// Server represents a gateway server on the Node.
type Server struct {
	srv      *http.Server
	srvMux   *mux.Router // http request multiplexer
	listener net.Listener

	started atomic.Bool
}

// NewServer returns a new gateway Server.
func NewServer(address, port string) *Server {
	srvMux := mux.NewRouter()
	srvMux.Use(setContentType)

	server := &Server{
		srvMux: srvMux,
	}
	server.srv = &http.Server{
		Addr:    address + ":" + port,
		Handler: server,
		// the amount of time allowed to read request headers. set to the default 2 seconds
		ReadHeaderTimeout: 2 * time.Second,
	}
	return server
}

// Start starts the gateway Server, listening on the given address.
func (s *Server) Start(context.Context) error {
	couldStart := s.started.CompareAndSwap(false, true)
	if !couldStart {
		log.Warn("cannot start server: already started")
		return nil
	}
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	s.listener = listener
	log.Infow("server started", "listening on", s.srv.Addr)
	//nolint:errcheck
	go s.srv.Serve(listener)
	return nil
}

// Stop stops the gateway Server.
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

// RegisterMiddleware allows to register a custom middleware that will be called before
// http.Request will reach handler.
func (s *Server) RegisterMiddleware(middlewareFuncs ...mux.MiddlewareFunc) {
	for _, m := range middlewareFuncs {
		// `router.Use` appends new middleware to existing
		s.srvMux.Use(m)
	}
}

// RegisterHandlerFunc registers the given http.HandlerFunc on the Server's multiplexer
// on the given pattern.
func (s *Server) RegisterHandlerFunc(pattern string, handlerFunc http.HandlerFunc, method string) {
	s.srvMux.HandleFunc(pattern, handlerFunc).Methods(method)
}

// ServeHTTP serves inbound requests on the Server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.srvMux.ServeHTTP(w, r)
}

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
