package p2p

import (
	"crypto/rand"
	"errors"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

const keyName = "p2p-key"

// Key provides a networking private key and PeerID of the node.
func Key(kstore keystore.Keystore) (crypto.PrivKey, error) {
	ksPriv, err := kstore.Get(keyName)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			// No existing private key in the keystore so generate a new one
			priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
			if err != nil {
				return nil, err
			}

			bytes, err := crypto.MarshalPrivateKey(priv)
			if err != nil {
				return nil, err
			}

			ksPriv = keystore.PrivKey{Body: bytes}
			err = kstore.Put(keyName, ksPriv)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return crypto.UnmarshalPrivateKey(ksPriv.Body)
}

func id(key crypto.PrivKey, pstore peerstore.Peerstore) (peer.ID, error) {
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return "", err
	}

	err = pstore.AddPrivKey(id, key)
	if err != nil {
		return "", err
	}

	return id, pstore.AddPubKey(id, key.GetPublic())
}
