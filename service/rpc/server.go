package rpc

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

// Server represents an RPC server on the Node.
// TODO @renaynay: eventually, rpc server should be able to be toggled on and off.
type Server struct {
	cfg Config

	srv      *http.Server
	srvMux   *mux.Router // http request multiplexer
	listener net.Listener
}

// NewServer returns a new RPC Server.
func NewServer(cfg Config) *Server {
	srvMux := mux.NewRouter()

	server := &Server{
		cfg:    cfg,
		srvMux: srvMux,
	}
	server.srv = &http.Server{
		Handler: server,
	}
	return server
}

// Start starts the RPC Server, listening on the given address.
func (s *Server) Start(context.Context) error {
	listenAddr := fmt.Sprintf("%s:%s", s.cfg.Address, s.cfg.Port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener
	log.Infow("RPC server started", "listening on", listener.Addr().String())
	//nolint:errcheck
	go s.srv.Serve(listener)
	return nil
}

// Stop stops the RPC Server.
func (s *Server) Stop(context.Context) error {
	// if server already stopped, return
	if s.listener == nil {
		return nil
	}
	if err := s.listener.Close(); err != nil {
		return err
	}
	s.listener = nil
	log.Info("RPC server stopped")
	return nil
}

// RegisterMiddleware allows to register a custom middleware that will be called before http.Request will reach handler
func (s *Server) RegisterMiddleware(m mux.MiddlewareFunc) {
	// `router.Use` appends new middleware to existing
	s.srvMux.Use(m)
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
	return s.listener.Addr().String()
}
