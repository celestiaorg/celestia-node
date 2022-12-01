package node

import (
	"context"
	"fmt"
	"runtime"

	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"
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

type admin struct {
	tp Type
}

func newAdmin(tp Type) Module {
	return &admin{
		tp: tp,
	}
}

func (a *admin) Type(context.Context) Type {
	return a.tp
}

// Version represents all binary build information.
type Version struct {
	SemanticVersion string `json:"semantic_version"`
	LastCommit      string `json:"last_commit"`
	BuildTime       string `json:"build_time"`
	SystemVersion   string `json:"system_version"`
	GoVersion       string `json:"go_version"`
}

func (a *admin) Version(context.Context) Version {
	return Version{
		SemanticVersion: semanticVersion,
		LastCommit:      lastCommit,
		BuildTime:       buildTime,
		SystemVersion:   fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS),
		GoVersion:       runtime.Version(),
	}
}

func (a *admin) LogLevelSet(_ context.Context, name, level string) error {
	return logging.SetLogLevel(name, level)
}

func (a *admin) AuthVerify(ctx context.Context, token string) ([]auth.Permission, error) {
	return []auth.Permission{}, fmt.Errorf("not implemented")
}

func (a *admin) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
