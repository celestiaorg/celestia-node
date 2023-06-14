//go:build darwin || freebsd || linux

package keystore

import (
	"fmt"
	"os"
)

// keyAccess checks whether file is not accessible by all users.
func keyAccess(path string) error {
	st, _ := os.Stat(path)
	mode := st.Mode()
	if mode&0077 != 0 {
		return fmt.Errorf("required: 0600, got: %#o", mode)
	}

	return nil
}
