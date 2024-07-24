package perms

import (
	"crypto/rand"
	"encoding/json"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
)

var (
	DefaultPerms   = []auth.Permission{"public"}
	ReadPerms      = []auth.Permission{"public", "read"}
	ReadWritePerms = []auth.Permission{"public", "read", "write"}
	AllPerms       = []auth.Permission{"public", "read", "write", "admin"}
)

var AuthKey = "Authorization"

// JWTPayload is a utility struct for marshaling/unmarshalling
// permissions into for token signing/verifying.
type JWTPayload struct {
	Allow []auth.Permission
	Nonce []byte
}

func (j *JWTPayload) MarshalBinary() (data []byte, err error) {
	return json.Marshal(j)
}

// NewTokenWithPerms generates and signs a new JWT token with the given secret
// and given permissions.
func NewTokenWithPerms(signer jwt.Signer, perms []auth.Permission) ([]byte, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}

	p := &JWTPayload{
		Allow: perms,
		Nonce: nonce[:],
	}
	token, err := jwt.NewBuilder(signer).Build(p)
	if err != nil {
		return nil, err
	}
	return token.Bytes(), nil
}
