package eds

import "fmt"

// DataHash is a representation of the share.Root hash.
type DataHash []byte

func (dh DataHash) Validate() error {
	if len(dh) != 32 {
		return fmt.Errorf("invalid hash size, expected 32, got %d", len(dh))
	}
	return nil
}

func (dh DataHash) String() string {
	c := dh
	return fmt.Sprintf("%X", c)
}
