package file

import "errors"

// ErrEmptyFile signals that the ODS file is empty.
// This helps avoid storing empty block EDSes.
var ErrEmptyFile = errors.New("file is empty")

const (
	// writeBufferSize defines buffer size for optimized batched writes into the file system.
	// TODO(@Wondertan): Consider making it configurable
	writeBufferSize = 64 << 10
	filePermissions = 0o600
)
