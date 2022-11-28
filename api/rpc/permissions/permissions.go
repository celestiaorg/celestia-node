package permissions

import "github.com/filecoin-project/go-jsonrpc/auth"

var (
	DefaultPerms   = []auth.Permission{"read"}
	ReadWritePerms = []auth.Permission{"read", "write"}
	AllPerms       = []auth.Permission{"read", "write", "admin"}
)

// JWTPayload is a utility struct for marshaling/unmarshalling
// permissions into for token signing/verifying.
type JWTPayload struct {
	Allow []auth.Permission
}
