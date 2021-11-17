package p2p

import (
	"crypto/rand"
	"errors"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

// TODO(Josh): What should the real name for this be?
const keyName = "test-key"

// TODO(@Wondertan): Should also receive a KeyStore to save generated key and reuse if exists.
// Identity provides a networking private key and PeerID of the node.
func Identity(pstore peerstore.Peerstore, ks keystore.Keystore) (crypto.PrivKey, peer.ID, error) {
	ksPriv, err := ks.Get(keyName)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			// No existing private key in the keystore so generate a new one
			genPriv, _, err := crypto.GenerateEd25519Key(rand.Reader)
			if err != nil {
				return nil, "", err
			}
			tempPriv, err := crypto.MarshalPrivateKey(genPriv)
			if err != nil {
				return nil, "", err
			}
			// Store the new private key in the keystore
			err = ks.Put(keyName, keystore.PrivKey{Body: tempPriv})
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
	}

	priv, err := crypto.UnmarshalPrivateKey(ksPriv.Body)
	if err != nil {
		return nil, "", err
	}

	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}

	err = pstore.AddPrivKey(id, priv)
	if err != nil {
		return nil, "", err
	}

	return priv, id, pstore.AddPubKey(id, priv.GetPublic())
}
