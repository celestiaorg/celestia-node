package file

import (
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("store/file")

const (
	// writeBufferSize defines buffer size for optimized batched writes into the file system.
	// TODO(@Wondertan): Consider making it configurable
	writeBufferSize = 64 << 10
	filePermissions = 0o600
)
