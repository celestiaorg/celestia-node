package p2p

import (
	"crypto/rand"
	"errors"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

// TODO(Josh): What should the real name for this be?
const keyName = "test-key"

// TODO(@Wondertan): Should also receive a KeyStore to save generated key and reuse if exists.
// Identity provides a networking private key and PeerID of the node.
func Identity(pstore peerstore.Peerstore, ks keystore.Keystore) (priv crypto.PrivKey, id peer.ID, err error) {
	ksPriv, err := ks.Get(keyName)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			// No existing private key in the keystore so generate a new one
			priv, _, err = crypto.GenerateEd25519Key(rand.Reader)
			if err != nil {
				return nil, "", err
			}
			bytes, err := crypto.MarshalPrivateKey(priv)
			if err != nil {
				return nil, "", err
			}
			// Store the new private key in the keystore
			err = ks.Put(keyName, keystore.PrivKey{Body: bytes})
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
	} else {
		priv, err = crypto.UnmarshalPrivateKey(ksPriv.Body)
		if err != nil {
			return nil, "", err
		}
	}

	id, err = peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}

	err = pstore.AddPrivKey(id, priv)
	if err != nil {
		return nil, "", err
	}

	return priv, id, pstore.AddPubKey(id, priv.GetPublic())
}
