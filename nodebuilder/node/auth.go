package node

import (
	"crypto/rand"
	"io"

	"github.com/cristalhq/jwt/v5"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

var SecretName = keystore.KeyName("jwt-secret.jwt")

// jwtSignerAndVerifier returns the node's JWT signer and verifier for a saved key,
// or generates and saves a new one if it does not.
func jwtSignerAndVerifier(ks keystore.Keystore) (jwt.Signer, jwt.Verifier, error) {
	key, ok := existing(ks)
	if !ok {
		// otherwise, generate and save new priv key
		sk, err := io.ReadAll(io.LimitReader(rand.Reader, 32))
		if err != nil {
			return nil, nil, err
		}

		// save key
		err = ks.Put(SecretName, keystore.PrivKey{Body: sk})
		if err != nil {
			return nil, nil, err
		}
		key = sk
	}

	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	if err != nil {
		return nil, nil, err
	}

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	if err != nil {
		return nil, nil, err
	}
	return signer, verifier, nil
}

func existing(ks keystore.Keystore) ([]byte, bool) {
	sk, err := ks.Get(SecretName)
	if err != nil {
		return nil, false
	}
	return sk.Body, true
}
