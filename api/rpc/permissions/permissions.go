package permissions

import "github.com/filecoin-project/go-jsonrpc/auth"

var (
	DefaultPerms   = []auth.Permission{"public"}
	ReadWritePerms = []auth.Permission{"public", "read", "write"}
	AllPerms       = []auth.Permission{"public", "read", "write", "admin"}
)

// JWTPayload is a utility struct for marshaling/unmarshalling
// permissions into for token signing/verifying.
type JWTPayload struct {
	Allow []auth.Permission
}
