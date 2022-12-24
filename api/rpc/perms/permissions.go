package perms

import (
	"encoding/json"

	"github.com/cristalhq/jwt"
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
}

func (j *JWTPayload) MarshalBinary() (data []byte, err error) {
	return json.Marshal(j)
}

// NewTokenWithPerms generates and signs a new JWT token with the given secret
// and given permissions.
func NewTokenWithPerms(secret jwt.Signer, perms []auth.Permission) ([]byte, error) {
	p := &JWTPayload{
		Allow: perms,
	}
	return jwt.NewTokenBuilder(secret).BuildBytes(p)
}
