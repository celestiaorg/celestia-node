package core

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const xtokenFileName = "xtoken.json"

func EmptyTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

// XToken retrieves the authentication token from a JSON file at the specified path.
func XToken(xtokenPath string) (string, error) {
	xtokenPath = filepath.Join(xtokenPath, xtokenFileName)
	exist := utils.Exists(xtokenPath)
	if !exist {
		return "", os.ErrNotExist
	}

	token, err := os.ReadFile(xtokenPath)
	if err != nil {
		return "", err
	}

	auth := struct {
		Token string `json:"x-token"`
	}{}

	err = json.Unmarshal(token, &auth)
	if err != nil {
		return "", err
	}
	if auth.Token == "" {
		return "", errors.New("x-token is empty. Please setup a token or cleanup xtokenPath")
	}
	return auth.Token, nil
}
