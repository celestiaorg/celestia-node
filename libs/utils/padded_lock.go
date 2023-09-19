package utils

import (
	"sync"

	"golang.org/x/sys/cpu"
)

// PaddedLock is -as the name indicates- a lock which is padded in memory
// exactly `cpu.CacheLinePad` length.
// This is used for stripped locking to avoid-false sharing scenarios.
type PaddedLock struct {
	sync.Mutex
	_ cpu.CacheLinePad
}
