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

func EmptyTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

type AuthToken struct {
	Token string `json:"x-token"`
}

// XToken retrieves the authentication token from a JSON file at the specified path.
// It first constructs the full file path by joining the provided directory path with the token file name.
// If the file does not exist, it returns an os.ErrNotExist error.
// If the file exists, it reads the content and unmarshalls it.
// If the field in the unmarshalled struct is empty, an error is returned indicating that the token is missing.
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
