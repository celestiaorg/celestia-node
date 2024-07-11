package p2p

import (
	"crypto/tls"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// TLSPath is an alias of the file path of TLS certificates and keys.
type TLSPath string

var (
	cert = "/cert.pem"
	key  = "/key.pem"
)

func tlsConfig(cfg *Config, path string) (*tls.Config, error) {
	exist := utils.Exists(path+cert) && utils.Exists(path+key)
	if !exist {
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil
	}

	cfg.upgrade()
	cert, err := tls.LoadX509KeyPair(path+cert, path+key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
}
