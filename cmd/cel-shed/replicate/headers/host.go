package headers

import (
	"crypto/rand"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
)

// NewReplicatorHost spins up an ephemeral libp2p host with no listeners. The
// replicator only dials outbound to the trusted source peer.
func NewReplicatorHost() (host.Host, error) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	return libp2p.New(
		libp2p.Identity(priv),
		libp2p.NoListenAddrs,
		libp2p.UserAgent("cel-shed/replicate"),
	)
}
