package node

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

var (
	buildTime       string
	lastCommit      string
	semanticVersion string
)

// Module defines the API related to interacting with the "administrative"
// node.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Type returns the node type.
	Type(context.Context) Type
	// Version returns information about the current binary build.
	Version(context.Context) Version

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
		Type        func(context.Context) Type                                         `perm:"admin"`
		Version     func(context.Context) Version                                      `perm:"admin"`
		LogLevelSet func(ctx context.Context, name, level string) error                `perm:"admin"`
		AuthVerify  func(ctx context.Context, token string) ([]auth.Permission, error) `perm:"admin"`
		AuthNew     func(ctx context.Context, perms []auth.Permission) ([]byte, error) `perm:"admin"`
	}
}

func (api *API) Type(ctx context.Context) Type {
	return api.Internal.Type(ctx)
}

func (api *API) Version(ctx context.Context) Version {
	return api.Internal.Version(ctx)
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
