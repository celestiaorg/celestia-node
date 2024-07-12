package p2p

import (
	cfg "crypto/tls"

	"github.com/libp2p/go-libp2p"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	cert = "/cert.pem"
	key  = "/key.pem"
)

// TLSPath is an alias of the file path of TLS certificates and keys.
type TLSPath string

type tls struct {
	*cfg.Config
	ListenAddresses     []string
	NoAnnounceAddresses []string
}

func newTLS(path string) (*tls, error) {
	var certificates []cfg.Certificate
	if path != "" {
		cert, err := cfg.LoadX509KeyPair(path+cert, path+key)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, cert)
	}
	config := &cfg.Config{MinVersion: cfg.VersionTLS12, Certificates: certificates}

	return &tls{
		Config: config,
		ListenAddresses: []string{
			"/dns4/0.0.0.0/tcp/2122/wss",
			"/dns6/[::]/tcp/2122/wss",
		},
		NoAnnounceAddresses: []string{
			"/dns4/127.0.0.1/tcp/2122/wss",
			"/dns6/[::]/tcp/2122/wss",
		},
	}, nil
}

func tlsConfig(path string) (*tls, error) {
	exist := utils.Exists(path+cert) && utils.Exists(path+key)
	if !exist {
		return newTLS("")
	}

	return newTLS(path)
}

func (tls *tls) upgrade(cfg *Config) {
	if len(tls.Certificates) == 0 {
		return
	}

	cfg.ListenAddresses = append(cfg.ListenAddresses, tls.ListenAddresses...)
	cfg.NoAnnounceAddresses = append(cfg.NoAnnounceAddresses, tls.NoAnnounceAddresses...)
}

func (tls *tls) transport() libp2p.Option {
	if len(tls.Config.Certificates) == 0 {
		return nil
	}
	return libp2p.Transport(ws.New, ws.WithTLSConfig(tls.Config))
}
