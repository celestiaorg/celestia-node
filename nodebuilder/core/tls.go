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

// TLS creates a TLS configuration using the certificate and key files from the specified path.
// It constructs the full paths to the certificate and key files by joining the provided directory path
// with their respective file names.
// If either file is missing, it returns an os.ErrNotExist error.
// If the files exist, it loads the X.509 key pair from the specified files and sets up a tls.Config
// with a minimum version of TLS 1.2.
// Parameters:
// * tlsPath: The directory path where the TLS certificate ("cert.pem") and key ("key.pem") files are located.
// Returns:
// * A tls.Config structure configured with the provided certificate and key.
// * An error if the certificate or key file does not exist, or if loading the key pair fails.
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

// XToken retrieves the authentication token from a JSON file at the specified path.
// It first constructs the full file path by joining the provided directory path with the token file name.
// If the file does not exist, it returns an os.ErrNotExist error.
// If the file exists, it reads the content and unmarshals it.
// If the field in the unmarshaled struct is empty, an error is returned indicating that the token is missing.
// Parameters:
// * xtokenPath: The directory path where the JSON file containing the X-Token is located.
// Returns:
// * The X-Token as a string if successfully retrieved.
// * An error if the file does not exist, reading fails, unmarshalling fails, or the token is empty.
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
