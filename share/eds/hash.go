package eds

import "fmt"

type DataHash []byte

func (dh DataHash) Validate() error {
	if len(dh) != 32 {
		return fmt.Errorf("invalid hash size, expected 32, got %d", len(dh))
	}
	return nil
}
