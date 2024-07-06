package utils

import (
	"io"

	logging "github.com/ipfs/go-log/v2"
)

func CloseAndLog(log logging.StandardLogger, name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnf("closing %s: %s", name, err)
	}
}
