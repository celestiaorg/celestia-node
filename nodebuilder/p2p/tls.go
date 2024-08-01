package p2p

import (
	cfg "crypto/tls"
	"github.com/libp2p/go-libp2p"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"os"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var (
	tlsPath = "TLS_PATH"
)

const (
	cert = "/cert.pem"
	key  = "/key.pem"
)

func enableWss() (libp2p.Option, bool, error) {
	path := os.Getenv(tlsPath)
	exist := utils.Exists(path+cert) && utils.Exists(path+key)
	if !exist {
		return libp2p.Transport(ws.New), exist, nil
	}

	var certificates []cfg.Certificate
	if path != "" {
		cert, err := cfg.LoadX509KeyPair(path+cert, path+key)
		if err != nil {
			return nil, false, err
		}
		certificates = append(certificates, cert)
	}
	config := &cfg.Config{MinVersion: cfg.VersionTLS12, Certificates: certificates}

	return libp2p.Transport(ws.New, ws.WithTLSConfig(config)), true, nil
}
