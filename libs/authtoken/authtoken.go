package authtoken

import (
	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
)

func NewSignedJWT(signer jwt.Signer, permissions []auth.Permission) (string, error) {
	token, err := jwt.NewTokenBuilder(signer).Build(&perms.JWTPayload{
		Allow: permissions,
	})
	if err != nil {
		return "", err
	}
	return token.InsecureString(), nil
}
