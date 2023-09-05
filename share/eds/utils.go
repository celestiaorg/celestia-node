package eds

import "io"

func logClose(name string, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Warnw("closing "+name, "err", err)
	}
}
