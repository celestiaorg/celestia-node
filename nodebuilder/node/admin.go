package node

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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
	revoker  *Revoker
}

func newModule(tp Type, signer jwt.Signer, verifier jwt.Verifier, revoker *Revoker) Module {
	return &module{
		tp:       tp,
		signer:   signer,
		verifier: verifier,
		revoker:  revoker,
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
	p, err := authtoken.ExtractSignedPayload(m.verifier, token)
	if err != nil {
		return nil, err
	}
	if m.revoker.IsRevoked(p.Nonce) {
		return nil, errors.New("token revoked")
	}
	return p.Allow, nil
}

func (m *module) AuthNew(_ context.Context, permissions []auth.Permission) (string, error) {
	return authtoken.NewSignedJWT(m.signer, permissions, 0)
}

func (m *module) AuthNewWithExpiry(_ context.Context,
	permissions []auth.Permission, ttl time.Duration,
) (string, error) {
	return authtoken.NewSignedJWT(m.signer, permissions, ttl)
}

func (m *module) AuthRevoke(_ context.Context, token string) error {
	p, err := authtoken.ExtractSignedPayload(m.verifier, token)
	if err != nil {
		return err
	}
	if len(p.Nonce) == 0 {
		return errors.New("token carries no nonce and cannot be individually revoked; rotate the signing key instead")
	}
	return m.revoker.Revoke(p.Nonce)
}

func (m *module) AuthRevokeNonce(_ context.Context, nonceHex string) error {
	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return fmt.Errorf("invalid nonce hex: %w", err)
	}
	return m.revoker.Revoke(nonce)
}

func (m *module) AuthRevoked(_ context.Context) ([]string, error) {
	return m.revoker.List(), nil
}
