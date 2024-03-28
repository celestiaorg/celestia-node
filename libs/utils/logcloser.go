package utils

import (
	"io"

	"github.com/ipfs/go-log/v2"
)

func CloseAndLog(log log.StandardLogger, name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnf("closing %s: %s", name, err)
	}
}
