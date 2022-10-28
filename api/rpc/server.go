package rpc

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rpc")

type Server struct {
	http    *http.Server
	rpc     *jsonrpc.RPCServer
	started atomic.Bool
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
	if s.started.CompareAndSwap(false, true) {
		//nolint:errcheck
		go s.http.ListenAndServe()
		s.started.Store(true)
		log.Infow("rpc: server started", "listening on", s.http.Addr)
		return nil
	}
	log.Warn("rpc: cannot start server: already started")
	return nil
}

// Stop stops the RPC Server.
func (s *Server) Stop(ctx context.Context) error {
	if s.started.CompareAndSwap(true, false) {
		err := s.http.Shutdown(ctx)
		if err != nil {
			return err
		}
		log.Info("rpc: server stopped")
		s.started.Store(false)
		return nil
	}
	log.Warn("rpc: cannot stop server: already stopped")
	return nil
}

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	return s.http.Addr
}
