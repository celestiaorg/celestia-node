package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("header-service")

// Service represents the header service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, P2PExchangeServer, Syncer, and so forth.
type Service struct {
	ex Exchange

	syncer        *Syncer
	p2pSubscriber *P2PSubscriber
	p2pServer     *P2PExchangeServer
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *Syncer,
	p2pSub *P2PSubscriber,
	p2pServer *P2PExchangeServer,
	ex Exchange) *Service {
	return &Service{
		syncer:        syncer,
		p2pSubscriber: p2pSub,
		p2pServer:     p2pServer,
		ex:            ex,
	}
}

// Start starts the header Service.
func (s *Service) Start(context.Context) error {
	log.Info("starting header service")
	return nil
}

// Stop stops the header Service.
func (s *Service) Stop(context.Context) error {
	log.Info("stopping header service")
	return nil
}
