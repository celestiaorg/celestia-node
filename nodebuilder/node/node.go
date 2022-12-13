package node

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

// Module defines the API related to interacting with the "administrative"
// node.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Info returns administrative information about the node.
	Info(context.Context) (Info, error)

	// LogLevelSet sets the given component log level to the given level.
	LogLevelSet(ctx context.Context, name, level string) error

	// AuthVerify returns the permissions assigned to the given token.
	AuthVerify(ctx context.Context, token string) ([]auth.Permission, error)
	// AuthNew signs and returns a new token with the given permissions.
	AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error)
}

var _ Module = (*API)(nil)

type API struct {
	Internal struct {
		Info        func(context.Context) (Info, error)                                `perm:"admin"`
		LogLevelSet func(ctx context.Context, name, level string) error                `perm:"admin"`
		AuthVerify  func(ctx context.Context, token string) ([]auth.Permission, error) `perm:"admin"`
		AuthNew     func(ctx context.Context, perms []auth.Permission) ([]byte, error) `perm:"admin"`
	}
}

func (api *API) Info(ctx context.Context) (Info, error) {
	return api.Internal.Info(ctx)
}

func (api *API) LogLevelSet(ctx context.Context, name, level string) error {
	return api.Internal.LogLevelSet(ctx, name, level)
}

func (api *API) AuthVerify(ctx context.Context, token string) ([]auth.Permission, error) {
	return api.Internal.AuthVerify(ctx, token)
}

func (api *API) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	return api.Internal.AuthNew(ctx, perms)
}
