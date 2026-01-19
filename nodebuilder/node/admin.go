package node

import (
	"context"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

var APIVersion = GetBuildInfo().SemanticVersion

type module struct {
	tp       Type
	signer   jwt.Signer
	verifier jwt.Verifier
}

func newModule(tp Type, signer jwt.Signer, verifier jwt.Verifier) Module {
	return &module{
		tp:       tp,
		signer:   signer,
		verifier: verifier,
	}
}

// Info contains information related to the administrative
// node.
type Info struct {
	Type       string `json:"type"`
	APIVersion string `json:"api_version"`
}

func (m *module) Info(context.Context) (Info, error) {
	return Info{
		Type:       m.tp.String(),
		APIVersion: APIVersion,
	}, nil
}

func (m *module) Ready(context.Context) (bool, error) {
	// Because the node uses FX to provide the RPC last, all services' lifecycles have been started by
	// the point this endpoint is available. It is not 100% guaranteed at this point that all services
	// are fully ready, but it is very high probability and all endpoints are available at this point
	// regardless.
	return true, nil
}

func (m *module) LogLevelSet(_ context.Context, name, level string) error {
	return logging.SetLogLevel(name, level)
}

func (m *module) AuthVerify(_ context.Context, token string) ([]auth.Permission, error) {
	return authtoken.ExtractSignedPermissions(m.verifier, token)
}

func (m *module) AuthNew(_ context.Context, permissions []auth.Permission) (string, error) {
	return authtoken.NewSignedJWT(m.signer, permissions, 0)
}

func (m *module) AuthNewWithExpiry(_ context.Context,
	permissions []auth.Permission, ttl time.Duration,
) (string, error) {
	return authtoken.NewSignedJWT(m.signer, permissions, ttl)
}
