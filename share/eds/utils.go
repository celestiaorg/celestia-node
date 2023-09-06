package eds

import "io"

func closeAndLog(name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnw("closing "+name, "err", err)
	}
}
