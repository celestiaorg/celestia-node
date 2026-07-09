package replicate

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store"
)

// putBlock writes a random EDS at the given height via the store (byte-identical
// to a node) and returns its DataHash.
func putBlock(t *testing.T, st *store.Store, height uint64, odsSize int) share.DataHash {
	t.Helper()
	square := edstest.RandEDS(t, odsSize)
	roots, err := share.NewAxisRoots(square)
	if err != nil {
		t.Fatalf("roots: %v", err)
	}
	if err := st.PutODSQ4(context.Background(), roots, height, square); err != nil {
		t.Fatalf("put height %d: %v", height, err)
	}
	return share.DataHash(roots.Hash())
}

// TestRunVerify covers the happy path (a real store-written block hashes to its
// header and matches its height hardlink), the empty-EDS symlink branch, and
// detection of on-disk data corruption.
func TestRunVerify(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Stop(ctx)

	putBlock(t, st, 10, 4)
	hash11 := putBlock(t, st, 11, 8)

	// An empty-EDS height links (as a symlink) to the canonical empty ODS that
	// NewStore populated.
	empty := share.DataHash(share.EmptyEDSDataHash())
	if _, err := ensureStoreLink(
		base+"/blocks", base+"/blocks/heights", 12, empty,
	); err != nil {
		t.Fatalf("empty link: %v", err)
	}

	cfg := VerifyConfig{DataDir: base, FromHeight: 10, ToHeight: 12, LogLevel: "error"}
	if err := RunVerify(ctx, cfg); err != nil {
		t.Fatalf("clean data should verify, got: %v", err)
	}

	// Corrupt the last byte (share data) of height 11's block. Because the height
	// link is a hardlink to the same inode, this corrupts both views: the shares
	// no longer hash to the DataHash stored in the header.
	blockPath := base + "/blocks/" + hash11.String() + ".ods"
	corruptLastByte(t, blockPath)

	if err := RunVerify(ctx, cfg); err == nil {
		t.Fatal("expected verification to fail after corrupting a block, got nil")
	}

	// The failed height (11) is recorded in the failed file, one entry.
	failedPath := cfg.failedFilePath()
	data, err := os.ReadFile(failedPath)
	if err != nil {
		t.Fatalf("read failed file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "11\t") {
		t.Fatalf("failed file = %q, want a single line for height 11", string(data))
	}

	// Restricting the window to only the intact heights should pass again and
	// clear the stale failed file.
	if err := RunVerify(ctx, VerifyConfig{DataDir: base, FromHeight: 10, ToHeight: 10, LogLevel: "error"}); err != nil {
		t.Fatalf("intact height should still verify, got: %v", err)
	}
	if _, err := os.Stat(failedPath); !os.IsNotExist(err) {
		t.Fatalf("clean run should remove stale failed file, stat err = %v", err)
	}
}

// TestAdvanceWatermark pins the in-order, gapless milestone behavior: feeding
// heights in a shuffled order still yields every interval milestone exactly
// once, in ascending order, and never advances past an unfinished height.
func TestAdvanceWatermark(t *testing.T) {
	const (
		from     = 300000
		to       = 300020
		interval = 5
	)
	// A deterministic out-of-order arrival sequence covering [from..to].
	arrivals := []uint64{
		300002, 300000, 300001, 300005, 300004, // 300000 completes early, 300005 before 300003
		300003, 300010, 300009, 300008, 300007, 300006, // filling backwards toward the watermark
		300011, 300013, 300012, 300015, 300014,
		300016, 300017, 300019, 300018, 300020,
	}

	done := make(map[uint64]struct{})
	watermark := uint64(from)
	var got []uint64
	for _, h := range arrivals {
		done[h] = struct{}{}
		var ms []uint64
		watermark, ms = advanceWatermark(watermark, to, interval, done)
		got = append(got, ms...)
		// The watermark must never claim a height that has not arrived yet.
		if _, ok := done[watermark]; ok && watermark != to {
			t.Fatalf("watermark %d still marked done — should have advanced past it", watermark)
		}
	}

	want := []uint64{300000, 300005, 300010, 300015, 300020}
	if len(got) != len(want) {
		t.Fatalf("milestones = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("milestones = %v, want %v (ascending, no gaps)", got, want)
		}
	}
	if len(done) != 0 {
		t.Fatalf("done set not drained: %v", done)
	}
	if watermark != to {
		t.Fatalf("final watermark = %d, want %d", watermark, to)
	}
}

// TestAdvanceWatermarkStalls confirms a missing (still-in-flight) height holds
// the watermark and its milestone back until that height arrives.
func TestAdvanceWatermarkStalls(t *testing.T) {
	done := make(map[uint64]struct{})
	watermark := uint64(11) // 11 is not a milestone, so nothing should fire until the hole fills
	// Everything above the hole 12 arrives first; milestone 15 must NOT fire yet.
	for _, h := range []uint64{11, 13, 14, 15, 16} {
		done[h] = struct{}{}
	}
	var ms []uint64
	watermark, ms = advanceWatermark(watermark, 20, 5, done)
	if watermark != 12 || len(ms) != 0 {
		t.Fatalf("stalled advance: watermark=%d milestones=%v, want watermark=12 no milestones", watermark, ms)
	}
	// The hole fills; now the watermark jumps and milestone 15 fires (once).
	done[12] = struct{}{}
	watermark, ms = advanceWatermark(watermark, 20, 5, done)
	// Consumes 12..16; stops at 17 (the next height not yet finished).
	if watermark != 17 {
		t.Fatalf("post-fill watermark = %d, want 17", watermark)
	}
	if len(ms) != 1 || ms[0] != 15 {
		t.Fatalf("post-fill milestones = %v, want [15]", ms)
	}
}

func corruptLastByte(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	off := fi.Size() - 1
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, off); err != nil {
		t.Fatal(err)
	}
	buf[0] ^= 0xFF
	if _, err := f.WriteAt(buf, off); err != nil {
		t.Fatal(err)
	}
}
