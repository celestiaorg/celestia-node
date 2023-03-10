package node

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/api/rpc"
)

type module struct {
	tp Type
}

func newModule(tp Type) Module {
	return &module{
		tp: tp,
	}
}

// Info contains information related to the administrative
// node.
type Info struct {
	Type       Type   `json:"type"`
	APIVersion string `json:"api_version"`
}

func (m *module) Info(context.Context) (Info, error) {
	return Info{
		Type:       m.tp,
		APIVersion: rpc.Version,
	}, nil
}

func (m *module) LogLevelSet(_ context.Context, name, level string) error {
	return logging.SetLogLevel(name, level)
}

func (m *module) AuthVerify(ctx context.Context, token string) ([]auth.Permission, error) {
	return []auth.Permission{}, fmt.Errorf("not implemented")
}

func (m *module) AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
