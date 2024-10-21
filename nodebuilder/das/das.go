package das

import (
	"context"

	"github.com/celestiaorg/celestia-node/das"
)

var _ Module = (*API)(nil)

//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// SamplingStats returns the current statistics over the DA sampling process.
	SamplingStats(ctx context.Context) (das.SamplingStats, error)
	// WaitCatchUp blocks until DASer finishes catching up to the network head.
	WaitCatchUp(ctx context.Context) error
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		SamplingStats func(ctx context.Context) (das.SamplingStats, error) `perm:"read"`
		WaitCatchUp   func(ctx context.Context) error                      `perm:"read"`
	}
}

func (api *API) SamplingStats(ctx context.Context) (das.SamplingStats, error) {
	return api.Internal.SamplingStats(ctx)
}

func (api *API) WaitCatchUp(ctx context.Context) error {
	return api.Internal.WaitCatchUp(ctx)
}
