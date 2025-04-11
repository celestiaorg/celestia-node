package p2p

import (
	"crypto/tls"

	"github.com/caddyserver/certmagic"
	p2pForge "github.com/ipshipyard/p2p-forge/client"
	"github.com/libp2p/go-libp2p/core/peer"
)

// User-Agent to use during DNS-01 ACME challenge
const userAgent = "go-libp2p/celestia-node"

// setupAutoTLS attempts to obtain TLS certificates automatically using p2p-forge.
// It returns a TLS config if successful, or nil if AutoTLS is not enabled or fails.
func setupAutoTLS(peerId peer.ID, certstore certmagic.FileStorage) (*tls.Config, error) {
	// p2pforge is the AutoTLS client library.
	// The cert manager handles the creation and management of certificate
	certManager, err := p2pForge.NewP2PForgeCertMgr(
		// Configure CA ACME endpoint
		p2pForge.WithCAEndpoint(p2pForge.DefaultCAEndpoint),

		// Configure where to store certificate
		p2pForge.WithCertificateStorage(&certstore),

		// Configure logger to use
		p2pForge.WithLogger(&log.SugaredLogger),

		// User-Agent to use during DNS-01 ACME challenge
		p2pForge.WithUserAgent(userAgent),
	)
	// Handle errors
	if err != nil {
		return nil, err
	}

	// Start the cert manager
	certManager.Start()
	defer certManager.Stop()

	return certManager.TLSConfig(), nil
}
