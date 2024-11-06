package core

import (
	"crypto/tls"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"os"
	"path/filepath"
)

const (
	cert = "cert.pem"
	key  = "key.pem"
)

var tlsPath = "CELESTIA_GRPC_TLS_PATH"

// TLS tries to read `CELESTIA_GRPC_TLS_PATH` to get the tls path and configure the config
// with build certificate. In returns an empty config in case the path hasn't specified.
func TLS() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	path := os.Getenv(tlsPath)
	if path == "" {
		return cfg, nil
	}

	certPath := filepath.Join(path, cert)
	keyPath := filepath.Join(path, key)
	exist := utils.Exists(certPath) && utils.Exists(keyPath)
	if !exist {
		return cfg, nil
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = append(cfg.Certificates, cert)
	return cfg, nil
}
