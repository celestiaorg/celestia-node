package header

import (
	"context"
	"fmt"
	"reflect"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("header-service")

// TODO @renaynay: DOCUMENT
// Service represents the Header service that can be started / stopped on a node.
// Service contains 3 main functionalities:
// 		1. Listening for/requesting new ExtendedHeaders from the network.
// 		2. Verifying and serving ExtendedHeaders to the network.
// 		3. Storing/caching ExtendedHeaders.
type Service struct {
	lifecycles []Lifecycle // TODO @renaynay: how to manage which lifecycles are "started" / which ones can be stopped just with context cancellation?
	active     []Lifecycle // keeps track of active lifecycles

	ctx    context.Context
	cancel context.CancelFunc
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(lifecycles []Lifecycle) *Service {
	return &Service{
		lifecycles: lifecycles,
	}
}

// Start starts the header Service.
func (s *Service) Start(ctx context.Context) error {
	if s.cancel != nil {
		return fmt.Errorf("header: Service already started")
	}
	log.Info("starting header service")

	s.ctx, s.cancel = context.WithCancel(context.Background())

	// start all lifecycles and keep track of those already started
	for _, lifecycle := range s.lifecycles {
		if err := lifecycle.Start(s.ctx); err != nil {
			// stop all started services
			if stopErr := s.Stop(ctx); stopErr != nil {
				log.Errorw("header-services: stopping service", "err", err)
			}
			return err
		}
		s.active = append(s.active, lifecycle)
	}
	return nil
}

// Stop stops the header Service and stops all of its active lifecycles.
func (s *Service) Stop(context.Context) error {
	if s.cancel == nil {
		return fmt.Errorf("header: Service already stopped")
	}
	log.Info("stopping header service")

	s.cancel()

	return stopLifecycles(s.active)
}

// stopLifecycles stops all given lifecycles. // TODO @renaynay: figure out error handling for this
func stopLifecycles(lifecycles []Lifecycle) error {
	for _, lifecycle := range lifecycles {
		if err := lifecycle.Stop(context.Background()); err != nil {
			log.Errorw("header-service: stopping lifecycle", "lifecycle",
				reflect.TypeOf(lifecycle).String(), "err", err)
			return err
		}
	}
	return nil
}
