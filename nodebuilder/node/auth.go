package node

import (
	"crypto/rand"
	"io"

	"github.com/cristalhq/jwt"
)

// secret returns the node's JWT secret if it exists, or generates
// and saves a new one if it does not.
//
// TODO @renaynay:
//  1. implement checking for existing key
//  2. implement saving the generated key to disk
//     (if the secret needs to be generated)
func secret() (jwt.Signer, error) {
	sk, err := io.ReadAll(io.LimitReader(rand.Reader, 32))
	if err != nil {
		return nil, err
	}
	return jwt.NewHS256(sk)
}
