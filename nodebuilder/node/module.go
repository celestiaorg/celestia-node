package node

import (
	"github.com/cristalhq/jwt/v5"
	"github.com/ipfs/go-datastore"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/authtoken"
)

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func(signer jwt.Signer, verifier *authtoken.Verifier) Module {
			return newModule(tp, signer, verifier)
		}),
		fx.Provide(jwtSignerAndVerifier),
		fx.Provide(func(verifier jwt.Verifier, store datastore.Batching) *authtoken.Verifier {
			return authtoken.NewVerifier(verifier, store)
		}),
	)
}
