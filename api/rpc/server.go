package rpc

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
)

type Server struct {
	srv      *http.Server
	handler  *jsonrpc.RPCServer
	listener net.Listener
}

func NewServer(address string, port string) *Server {
	handler := jsonrpc.NewServer()
	return &Server{
		handler: handler,
		srv: &http.Server{
			Addr:    address + ":" + port,
			Handler: handler,
			// the amount of time allowed to read request headers. set to the default 2 seconds
			ReadHeaderTimeout: 2 * time.Second,
		},
	}
}

func (s *Server) Start(context.Context) error {
	listener, err := net.Listen("tcp", s.srv.Addr)
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

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	return s.listener.Addr().String()
}
