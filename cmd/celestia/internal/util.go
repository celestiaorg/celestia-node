package internal

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/celestiaorg/celestia-node/share"
)

func PrintOutput(data interface{}, err error, formatData func(interface{}) interface{}) error {
	switch {
	case err != nil:
		data = err
	case formatData != nil:
		data = formatData(data)
	}

	resp := struct {
		Result interface{} `json:"result"`
	}{
		Result: data,
	}

	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(bytes))
	return nil
}

// ParseV0Namespace parses a namespace from a base64 or hex string. The param
// is expected to be the user-specified portion of a v0 namespace ID (i.e. the
// last 10 bytes).
func ParseV0Namespace(param string) (share.Namespace, error) {
	userBytes, err := DecodeToBytes(param)
	if err != nil {
		return nil, err
	}

	// if the namespace ID is <= 10 bytes, left pad it with 0s
	return share.NewBlobNamespaceV0(userBytes)
}

// DecodeToBytes decodes a Base64 or hex input string into a byte slice.
func DecodeToBytes(param string) ([]byte, error) {
	if strings.HasPrefix(param, "0x") {
		decoded, err := hex.DecodeString(param[2:])
		if err != nil {
			return nil, fmt.Errorf("error decoding namespace ID: %w", err)
		}
		return decoded, nil
	}
	// otherwise, it's just a base64 string
	decoded, err := base64.StdEncoding.DecodeString(param)
	if err != nil {
		return nil, fmt.Errorf("error decoding namespace ID: %w", err)
	}
	return decoded, nil
}
