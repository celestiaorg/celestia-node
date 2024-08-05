package p2p

import (
	"crypto/tls"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	cert = "cert.pem"
	key  = "key.pem"
)

var tlsPath = "TLS_PATH"

// enableWss checks whether `tlsPath` is not empty and creates a certificates
// to enable a websocket transport.
func enableWss() (libp2p.Option, bool, error) {
	path := os.Getenv(tlsPath)
	certPath := filepath.Join(path, cert)
	keyPath := filepath.Join(path, key)

	exist := utils.Exists(certPath) && utils.Exists(keyPath)
	if !exist {
		return libp2p.Transport(ws.New), exist, nil
	}

	var certificates []tls.Certificate
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, false, err
	}
	certificates = append(certificates, cert)
	config := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: certificates}

	return libp2p.Transport(ws.New, ws.WithTLSConfig(config)), true, nil
}
