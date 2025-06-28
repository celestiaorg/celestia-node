package p2p

import (
	"crypto/tls"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var (
	cert = "cert.pem"
	key  = "key.pem"
)

var (
	tlsPath     = "CELESTIA_TLS_PATH"
	tlsKeyName  = "CELESTIA_TLS_KEY_FILENAME"
	tlsCertName = "CELESTIA_TLS_CERT_FILENAME"
)

// tlsEnabled checks whether `tlsPath` is not empty and creates a certificate.
// it returns the cfg itself, the bool flag that specifies whether the config was created
// and an error.
func tlsEnabled() (*tls.Config, bool, error) {
	path := os.Getenv(tlsPath)
	if path == "" {
		log.Debug("the CELESTIA_TLS_PATH was not set")
		return nil, false, nil
	}

	keyFileName := os.Getenv(tlsKeyName)
	if keyFileName != "" {
		key = keyFileName
	}

	certFileName := os.Getenv(tlsCertName)
	if certFileName != "" {
		cert = certFileName
	}

	certPath := filepath.Join(path, cert)
	keyPath := filepath.Join(path, key)
	exist := utils.Exists(certPath) && utils.Exists(keyPath)
	if !exist {
		log.Error("the TLS cert and/or key file were not found")
		return nil, false, nil
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, false, err
	}

	log.Info("TLS successfully loaded")

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}, true, nil
}

// wsTransport enables a support for the secure websocket connection
// using the passed tls config. The connection will be insecure in case
// config is empty.
func wsTransport(config *tls.Config) libp2p.Option {
	if config == nil {
		log.Info("using a default ws transport")
		return libp2p.Transport(ws.New)
	}

	log.Info("using a wss transport with tlsConfig")
	return libp2p.Transport(ws.New, ws.WithTLSConfig(config))
}
