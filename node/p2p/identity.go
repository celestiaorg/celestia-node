package p2p

import (
	"crypto/rand"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

// TODO(@Wondertan): Should also receive a KeyStore to save generated key and reuse if exists.
// Identity provides a networking private key and PeerID of the node.
func Identity(pstore peerstore.Peerstore) (crypto.PrivKey, peer.ID, error) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
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

	return priv, id, pstore.AddPubKey(id, pub)
}
