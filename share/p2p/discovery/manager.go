package discovery

import (
	"context"
	"errors"
)

type Manager struct {
	discs []*Discovery

	cancel context.CancelFunc
}

func NewManager(discs []*Discovery) *Manager {
	return &Manager{
		discs: discs,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	advertiseCtx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	for _, d := range m.discs {
		err := d.Start(ctx)
		if err != nil {
			log.Errorw("failed to start discovery", "err", err)

			m.cancel()
			return err
		}
		log.Infow("starting discovery", "topic", d.tag)

		if d.params.EnableAdvertise {
			log.Infow("advertising to topic", "topic", d.tag)
			go d.Advertise(advertiseCtx)
		}
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	// cancels advertisement if it is happening
	m.cancel()

	var err error
	for _, d := range m.discs {
		err = errors.Join(err, d.Stop(ctx))
	}
	return err
}
