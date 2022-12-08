package p2p

import (
	"context"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"time"
)

var log = logging.Logger("p2p/server")

// TODO: make functional
type Config struct {
	writeTimeout time.Duration
	readTimeout  time.Duration
	interceptors []ServerInterceptor
}

type Server struct {
	ctx                       context.Context
	host                      host.Host
	writeTimeout, readTimeout time.Duration
	interceptorsChain         ServerInterceptor
}

type Handle func(context.Context, *Session) error

type ServerInterceptor func(context.Context, *Session, Handle) error

func NewServer(cfg Config, host host.Host) Server {
	return Server{
		host:              host,
		writeTimeout:      cfg.writeTimeout,
		readTimeout:       cfg.readTimeout,
		interceptorsChain: chainServerInterceptors(cfg.interceptors...),
	}
}

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

func (srv *Server) RegisterHandler(pid protocol.ID, handler Handle) {
	h := srv.sessionHandler(handler)
	srv.host.SetStreamHandler(pid, h)
}

func (srv *Server) RemoveHandler(pid protocol.ID) {
	srv.host.RemoveStreamHandler(pid)
}

func (srv *Server) sessionHandler(h Handle) network.StreamHandler {
	return func(stream network.Stream) {
		s := srv.newSession(stream)
		err := srv.interceptorsChain(context.TODO(), s, h)
		if err != nil {
			log.Errorf("handle session: %s", err.Error())
			s.Stream.Reset() //nolint:errcheck
		}
	}
}

func (srv *Server) newSession(stream network.Stream) *Session {
	return &Session{
		Ctx:          srv.ctx,
		writeTimeout: srv.writeTimeout,
		readTimeout:  srv.readTimeout,
		Stream:       stream,
	}
}
