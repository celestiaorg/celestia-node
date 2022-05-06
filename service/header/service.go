package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headerexchange"
	"github.com/celestiaorg/celestia-node/header/headersync"
)

var log = logging.Logger("header_service")

// Service represents the header service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, P2PExchangeServer, Syncer, and so forth.
type Service struct {
	ex header.Exchange

	syncer    *headersync.Syncer
	sub       header.Subscriber
	p2pServer *headerexchange.P2PExchangeServer
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *headersync.Syncer,
	sub header.Subscriber,
	p2pServer *headerexchange.P2PExchangeServer,
	ex header.Exchange) *Service {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
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

// GetByHeight returns the ExtendedHeader at the given height, blocking
// until header has been processed by the store or context deadline is exceeded.
func (s *Service) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return s.syncer.GetByHeight(ctx, height)
}

// IsSyncing returns the status of sync
func (s *Service) IsSyncing() bool {
	return !s.syncer.State().Finished()
}
