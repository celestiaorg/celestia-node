package header

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type Hash []byte

func (h Hash) String() string {
	return strings.ToUpper(hex.EncodeToString(h))
}

func (h Hash) MarshalJSON() ([]byte, error) {
	s := strings.ToUpper(hex.EncodeToString(h))
	jbz := make([]byte, len(s)+2)
	jbz[0] = '"'
	copy(jbz[1:], s)
	jbz[len(jbz)-1] = '"'
	return jbz, nil
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hex string: %s", data)
	}
	bz2, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*h = bz2
	return nil
}
