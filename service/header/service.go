package header

import (
	"context"
	"fmt"
	"reflect"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("header-service")

// Service represents the header service that can be started / stopped on a node.
// Service's main function is to manage its sub-services' lifecycles. Service can
// contain several sub-services, such as Exchange, P2PServer, Syncer, and so forth.
type Service struct {
	// TODO @renaynay: how to manage which lifecycles are "started" / which ones can be stopped just with context
	//  cancellation?
	lifecycles []Lifecycle
	// TODO @renaynay: how does this keep track of lifecycles that stop themselves, such as Syncer?
	active []Lifecycle // keeps track of active lifecycles

	ctx    context.Context
	cancel context.CancelFunc
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(lifecycles []Lifecycle) *Service {
	return &Service{
		lifecycles: lifecycles,
	}
}

// Start starts the header Service and all of its lifecycles.
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
