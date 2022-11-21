package rpc

import (
	"context"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rpc")

var (
	AllPerms     = []auth.Permission{"read", "write", "admin"}
	DefaultPerms = []auth.Permission{"read"}
)

type Server struct {
	srv      *http.Server
	rpc      *jsonrpc.RPCServer
	listener net.Listener

	started atomic.Bool
}

func NewServer(address, port string) *Server {
	rpc := jsonrpc.NewServer()
	authHandler := &auth.Handler{
		Verify: func(ctx context.Context, token string) ([]auth.Permission, error) {
			// TODO(distractedm1nd/renaynay): implement auth
			log.Warn("auth not implemented, token: ", token)
			return DefaultPerms, nil
		},
		Next: rpc.ServeHTTP,
	}
	return &Server{
		rpc: rpc,
		srv: &http.Server{
			Addr:    address + ":" + port,
			Handler: authHandler,
			// the amount of time allowed to read request headers. set to the default 2 seconds
			ReadHeaderTimeout: 2 * time.Second,
		},
	}
}

// RegisterService registers a service onto the RPC server. All methods on the service will then be
// exposed over the RPC.
func (s *Server) RegisterService(namespace string, service interface{}) {
	s.rpc.Register(namespace, service)
}

// RegisterAuthedService registers a service onto the RPC server. All methods on the service will
// then be exposed over the RPC.
func (s *Server) RegisterAuthedService(namespace string, service interface{}, out interface{}) {
	auth.PermissionedProxy(AllPerms, DefaultPerms, service, getInternalStruct(out))
	s.RegisterService(namespace, out)
}

func getInternalStruct(api interface{}) interface{} {
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
		return err
	}
	s.listener = listener
	log.Infow("server started", "listening on", s.srv.Addr)
	//nolint:errcheck
	go s.srv.Serve(listener)
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
