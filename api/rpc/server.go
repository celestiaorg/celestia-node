package rpc

import (
	"context"
	"net/http"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rpc")

type Server struct {
	http *http.Server
	rpc  *jsonrpc.RPCServer
}

func NewServer(address string, port string) *Server {
	rpc := jsonrpc.NewServer()
	return &Server{
		rpc: rpc,
		http: &http.Server{
			Addr:    address + ":" + port,
			Handler: rpc,
			// the amount of time allowed to read request headers. set to the default 2 seconds
			ReadHeaderTimeout: 2 * time.Second,
		},
	}
}

// RegisterService registers a service onto the RPC server. All methods on the service will then be exposed over the
// RPC.
func (s *Server) RegisterService(namespace string, service interface{}) {
	s.rpc.Register(namespace, service)
}

// Start starts the RPC Server.
func (s *Server) Start(context.Context) error {
	//nolint:errcheck
	go s.http.ListenAndServe()
	log.Infow("RPC server started", "listening on", s.http.Addr)
	return nil
}

// Stop stops the RPC Server.
func (s *Server) Stop(ctx context.Context) error {
	// if server already stopped, return
	err := s.http.Shutdown(ctx)
	if err != nil {
		return err
	}
	log.Info("RPC server stopped")
	return nil
}

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	return s.http.Addr
}
