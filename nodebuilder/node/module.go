package node

import (
	"path/filepath"

	"github.com/cristalhq/jwt/v5"
	"go.uber.org/fx"
)

// RevokedTokensPath is the shared on-disk location for revoked JWT nonces (node + CLI).
func RevokedTokensPath(storePath string) string {
	return filepath.Join(storePath, "auth", "revoked.json")
}

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func(signer jwt.Signer, verifier jwt.Verifier, revoker *Revoker) Module {
			return newModule(tp, signer, verifier, revoker)
		}),
		fx.Provide(jwtSignerAndVerifier),
		fx.Provide(func(path StorePath) (*Revoker, error) {
			return NewRevoker(RevokedTokensPath(string(path)))
		}),
	)
}
