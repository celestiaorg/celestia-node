package core

import (
	"crypto/tls"
	"fmt"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	cert = "cert.pem"
	key  = "key.pem"
)

// TLS parses the tls path and tries to configure the config with tls certificates.
// In returns an empty config in case the path was not specified.
func TLS(tlsPath string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if tlsPath == "" {
		return cfg, nil
	}
	certPath := filepath.Join(tlsPath, cert)
	keyPath := filepath.Join(tlsPath, key)
	exist := utils.Exists(certPath) && utils.Exists(keyPath)
	if !exist {
		return nil, fmt.Errorf("can't find %s or %s under %s"+
			"Please specify another path or disable tls in the config",
			cert, key, tlsPath,
		)
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = append(cfg.Certificates, cert)
	return cfg, nil
}
