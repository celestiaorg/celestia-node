package authtoken

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

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

	if err := json.Unmarshal(tk.Claims(), p); err != nil {
		return nil, err
	}
	if !p.ExpiresAt.IsZero() && p.ExpiresAt.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("token expired %s ago", time.Since(p.ExpiresAt))
	}
	return p.Allow, nil
}

// NewSignedJWT returns a signed JWT token with the passed permissions and signer.
func NewSignedJWT(signer jwt.Signer, permissions []auth.Permission, ttl time.Duration) (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	var expiresAt time.Time
	if ttl != 0 {
		expiresAt = time.Now().UTC().Add(ttl)
	}

	token, err := jwt.NewBuilder(signer).Build(&perms.JWTPayload{
		Allow:     permissions,
		Nonce:     nonce,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", err
	}
	return token.String(), nil
}
