package authtoken

import (
	"encoding/json"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
)

// ExtractSignedPermissions returns the permissions granted to the token by the passed signer.
// If the token isn't signed by the signer, it will not pass verification.
func ExtractSignedPermissions(verifier jwt.Verifier, token string) ([]auth.Permission, error) {
	tk, err := jwt.Parse([]byte(token), verifier)
	if err != nil {
		return nil, err
	}
	p := new(perms.JWTPayload)
	err = json.Unmarshal(tk.Claims(), p)
	if err != nil {
		return nil, err
	}
	return p.Allow, nil
}

// NewSignedJWT returns a signed JWT token with the passed permissions and signer.
func NewSignedJWT(signer jwt.Signer, permissions []auth.Permission) (string, error) {
	token, err := jwt.NewBuilder(signer).Build(&perms.JWTPayload{
		Allow: permissions,
	})
	if err != nil {
		return "", err
	}
	return token.String(), nil
}
