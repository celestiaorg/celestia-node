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

func NewServer(address, port string) *Server {
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
	couldStart := s.started.CompareAndSwap(false, true)
	if !couldStart {
		log.Warn("cannot start server: already started")
		return nil
	}
	//nolint:errcheck
	go s.http.ListenAndServe()
	log.Infow("server started", "listening on", s.http.Addr)
	return nil
}

// Stop stops the RPC Server.
func (s *Server) Stop(ctx context.Context) error {
	couldStop := s.started.CompareAndSwap(true, false)
	if !couldStop {
		log.Warn("cannot stop server: already stopped")
		return nil
	}
	err := s.http.Shutdown(ctx)
	if err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}

// ListenAddr returns the listen address of the server.
func (s *Server) ListenAddr() string {
	return s.http.Addr
}
