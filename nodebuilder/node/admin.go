package node

import (
	"context"

	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

const APIVersion = "v0.2.1"

type module struct {
	tp     Type
	signer jwt.Signer
}

func newModule(tp Type, signer jwt.Signer) Module {
	return &module{
		tp:     tp,
		signer: signer,
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
		APIVersion: APIVersion,
	}, nil
}

func (m *module) LogLevelSet(_ context.Context, name, level string) error {
	return logging.SetLogLevel(name, level)
}

func (m *module) AuthVerify(_ context.Context, token string) ([]auth.Permission, error) {
	return authtoken.ExtractSignedPermissions(m.signer, token)
}

func (m *module) AuthNew(_ context.Context, permissions []auth.Permission) (string, error) {
	return authtoken.NewSignedJWT(m.signer, permissions)
}
