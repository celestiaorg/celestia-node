package utils

import (
	"io"

	logging "github.com/ipfs/go-log/v2"
)

// CloseAndLog closes the closer and logs any error that occurs. The function is handy wrapping
// to group closing and logging in one call for defer statements.
func CloseAndLog(log logging.StandardLogger, name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnf("closing %s: %s", name, err)
	}
}
