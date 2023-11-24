package perms

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"
)

type Permission int

const (
	Default Permission = iota
	Read
	ReadWrite
	Admin
)

var permissions = map[Permission][]auth.Permission{
	Default:   {"read"},
	Read:      {"read"},
	ReadWrite: {"read", "write"},
	Admin:     {"read", "write", "admin"},
}

var ErrInvalidPermissions = errors.New("invalid permission specific")

func With(p Permission) []auth.Permission {
	return permissions[p]
}

func PermissionsFromString(name string) ([]auth.Permission, error) {
	p, err := FromString(name)
	return permissions[p], err
}

// FromString converts a string to the corresponding Permission iota value.
func FromString(s string) (Permission, error) {
	switch s {
	case "default":
		return Default, nil
	case "read", "r":
		return Read, nil
	case "readwrite", "rw":
		return ReadWrite, nil
	case "admin":
		return Admin, nil
	default:
		return Default, fmt.Errorf("%s: %s", ErrInvalidPermissions, s)
	}
}

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
