package node

import (
	"context"
	"fmt"

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

// Info contains information related to the administrative
// node.
type Info struct {
	Type       Type   `json:"type"`
	APIVersion string `json:"api_version"`
}

func (a *admin) Info(context.Context) (Info, error) {
	return Info{
		Type: a.tp,
		// TODO @renaynay @distractedm1nd: Implement versioning in API and way to extract that into this struct
	}, nil
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
