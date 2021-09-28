package utils

import "os"

// Exists checks whether file or directory exists under the given 'path' on the system.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
