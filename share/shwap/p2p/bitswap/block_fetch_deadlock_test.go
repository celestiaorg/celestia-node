package bitswap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHasher_MutexReleasedOnUnmarshalError is a regression test ensuring
// hasher.write releases the per-entry mutex on the error path.
//
// When an UnmarshalFn returns an error (e.g. a peer sends a block with a valid
// CID and proto envelope but corrupted/invalid container data), hasher.write
// must still release entry.Lock(). The entry lives in the global unmarshalFns
// map, so a leaked lock is observable by every subsequent goroutine that loads
// the same entry, causing them to block forever on entry.Lock().
func TestHasher_MutexReleasedOnUnmarshalError(t *testing.T) {
	blk := newTestBlock(0xBEEF & 0xFFFF)
	protoData, err := marshalProto(blk)
	require.NoError(t, err)

	entry := &unmarshalEntry{
		UnmarshalFn: func(_, _ []byte) error {
			return fmt.Errorf("simulated invalid share data / forged proof")
		},
	}
	unmarshalFns.Store(blk.CID(), entry)
	defer unmarshalFns.Delete(blk.CID())

	h := &hasher{IDSize: testIDSize}
	_, writeErr := h.Write(protoData)
	require.Error(t, writeErr, "hasher.Write must reject invalid data")

	// After the error path, the entry mutex must NOT be held. TryLock returns
	// false if the mutex is still locked (the leak); true if it is free.
	free := entry.TryLock()
	require.True(t, free, "entry mutex still held after UnmarshalFn error — lock leaked")
	entry.Unlock()
}
