package node

import (
	"crypto/rand"
	"io"

	"github.com/cristalhq/jwt"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

var SecretName = keystore.KeyName("jwt-secret.jwt")

// secret returns the node's JWT secret if it exists, or generates
// and saves a new one if it does not.
func secret(ks keystore.Keystore) (jwt.Signer, error) {
	// if key already exists, use it
	if pk, ok := existing(ks); ok {
		return jwt.NewHS256(pk)
	}
	// otherwise, generate and save new priv key
	sk, err := io.ReadAll(io.LimitReader(rand.Reader, 32))
	if err != nil {
		return nil, err
	}
	// save key
	err = ks.Put(SecretName, keystore.PrivKey{Body: sk})
	if err != nil {
		return nil, err
	}

	return jwt.NewHS256(sk)
}

func existing(ks keystore.Keystore) ([]byte, bool) {
	sk, err := ks.Get(SecretName)
	if err != nil {
		return nil, false
	}
	return sk.Body, true
}
