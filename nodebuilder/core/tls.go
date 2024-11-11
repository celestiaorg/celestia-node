package core

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	cert   = "cert.pem"
	key    = "key.pem"
	xtoken = "xtoken.json"
)

// TLS parses the tls path and tries to configure the config with tls certificates.
// In returns an empty config in case the path was not specified.
func TLS(tlsPath string) (*tls.Config, error) {
	certPath := filepath.Join(tlsPath, cert)
	keyPath := filepath.Join(tlsPath, key)
	exist := utils.Exists(certPath) && utils.Exists(keyPath)
	if !exist {
		return nil, os.ErrNotExist
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = append(cfg.Certificates, cert)
	return cfg, nil
}

type AuthToken struct {
	Token string `json:"x-token"`
}

func XToken(xtokenPath string) (string, error) {
	xtokenPath = filepath.Join(xtokenPath, xtoken)
	exist := utils.Exists(xtokenPath)
	if !exist {
		return "", os.ErrNotExist
	}

	token, err := os.ReadFile(xtokenPath)
	if err != nil {
		return "", err
	}

	var auth AuthToken
	err = json.Unmarshal(token, &auth)
	if err != nil {
		return "", err
	}
	if auth.Token == "" {
		return "", errors.New("x-token is empty. Please setup a token or cleanup xtokenPath")
	}
	return auth.Token, nil
}
