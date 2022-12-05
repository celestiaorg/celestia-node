package node

import (
	"context"
	"fmt"
	"runtime"

	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"
)

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
