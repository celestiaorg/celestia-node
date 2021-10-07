package rpc

import (
	"net"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rpc")

// Server represents an RPC server on the Node.
// TODO @renaynay: eventually, rpc server should be able to be toggled on and off.
type Server struct {
	srv      *http.Server
	srvMux   *http.ServeMux // http request multiplexer
	listener net.Listener
}

// NewServer returns a new RPC Server.
func NewServer() *Server {
	server := &Server{
		srvMux: http.NewServeMux(),
	}
	server.srv = &http.Server{
		Handler: server,
	}
	return server
}

// Start starts the RPC Server, listening on the given address.
func (s *Server) Start(listenAddr string) error {
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
func (s *Server) Stop() error {
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

// RegisterHandler registers the given handler on the Server's multiplexer
// on the given pattern.
func (s *Server) RegisterHandler(pattern string, handler http.Handler) {
	s.srvMux.Handle(pattern, handler)
}

// ServeHTTP serves inbound requests on the Server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.srvMux.ServeHTTP(w, r)
}
