package p2p

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var log = logging.Logger("p2p/server")

// TODO: make functional
type Config struct {
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	Interceptors []ServerInterceptor
}

// Server is a utility wrapper around libp2p host that abstracts away common server functionality.
// Execution order of interceptors is from last to first
type Server struct {
	ctx                       context.Context
	host                      host.Host
	writeTimeout, readTimeout time.Duration
	interceptorsChain         ServerInterceptor
}

// Handle handles server side of communication
type Handle func(context.Context, *Session) error

// ServerInterceptor is the server side middleware
type ServerInterceptor func(context.Context, *Session, Handle) error

func NewServer(cfg Config, host host.Host) Server {
	return Server{
		host:              host,
		writeTimeout:      cfg.WriteTimeout,
		readTimeout:       cfg.ReadTimeout,
		interceptorsChain: chainServerInterceptors(cfg.Interceptors...),
	}
}

// chainServerInterceptors reduces multiple interceptors to one chain.Execution order of
// interceptors is from last to first.
func chainServerInterceptors(interceptors ...ServerInterceptor) ServerInterceptor {
	n := len(interceptors)
	return func(ctx context.Context, session *Session, handler Handle) error {
		chainer := func(currentInter ServerInterceptor, currentHandler Handle) Handle {
			return func(ctx context.Context, s *Session) error {
				return currentInter(ctx, s, currentHandler)
			}
		}

		chainedHandler := handler
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(interceptors[i], chainedHandler)
		}

		return chainedHandler(ctx, session)
	}
}

// RegisterHandler sets the handler on the Host's Mux
func (srv *Server) RegisterHandler(pid protocol.ID, handler Handle) {
	h := srv.sessionHandler(handler)
	srv.host.SetStreamHandler(pid, h)
}

// RemoveHandler removes a handler from the stream mux that was set by
// RegisterHandler
func (srv *Server) RemoveHandler(pid protocol.ID) {
	srv.host.RemoveStreamHandler(pid)
}

// sessionHandler creates handler with interceptors and session
func (srv *Server) sessionHandler(h Handle) network.StreamHandler {
	return func(stream network.Stream) {
		s := srv.newSession(stream)
		err := srv.interceptorsChain(context.Background(), s, h)
		if err != nil {
			log.Errorf("handle session: %s", err.Error())
			s.Stream.Reset() //nolint:errcheck
		}
	}
}

func (srv *Server) newSession(stream network.Stream) *Session {
	return &Session{
		writeTimeout: srv.writeTimeout,
		readTimeout:  srv.readTimeout,
		Stream:       stream,
	}
}
