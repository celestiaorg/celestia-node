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
	interceptors []Interceptor
}

type Server struct {
	ctx                       context.Context
	host                      host.Host
	writeTimeout, readTimeout time.Duration
	interceptorsChain         Interceptor
}

func NewServer(cfg Config, host host.Host) Server {
	chain := func(s *Session, handle Handle) error {
		return handle(s)
	}
	for _, i := range cfg.interceptors {
		chain = func(s *Session, handle Handle) error {
			return i(s, handle)
		}
	}

	return Server{
		host:              host,
		writeTimeout:      cfg.writeTimeout,
		readTimeout:       cfg.readTimeout,
		interceptorsChain: chain,
	}
}

func (srv *Server) RegisterHandler(h Handler) {
	handler := srv.sessionHandler(h)
	srv.host.SetStreamHandler(h.ProtocolID(), handler)
}

func (srv *Server) RemoveHandler(h Handler) {
	srv.host.RemoveStreamHandler(h.ProtocolID())
}

func (srv *Server) sessionHandler(h Handler) network.StreamHandler {
	return func(stream network.Stream) {
		s := srv.newSession(stream)
		err := srv.interceptorsChain(s, h.Handle)
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

type Interceptor func(session *Session, handle Handle) error

type Handle func(*Session) error

type Handler interface {
	ProtocolID() protocol.ID
	Handle(*Session) error
}
