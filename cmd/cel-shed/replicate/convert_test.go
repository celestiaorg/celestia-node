package replicate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

func linkPathFor(heightsDir string, height uint64) string {
	return filepath.Join(heightsDir, strconv.FormatUint(height, 10)+".ods")
}

// TestEnsureStoreLink pins the store linking convention: hardlink for non-empty
// (repairing a stray symlink), symlink for empty EDS, and idempotency.
func TestEnsureStoreLink(t *testing.T) {
	base := t.TempDir()
	blocksDir := filepath.Join(base, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	if err := os.MkdirAll(heightsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	nonEmpty := make([]byte, share.DataHashSize)
	for i := range nonEmpty {
		nonEmpty[i] = byte(i + 1)
	}
	hash := share.DataHash(nonEmpty)
	blockPath := filepath.Join(blocksDir, hash.String()+".ods")
	if err := os.WriteFile(blockPath, []byte("BLOCKDATA"), 0o644); err != nil {
		t.Fatal(err)
	}

	assertHardlink := func(t *testing.T, lp string) {
		t.Helper()
		li, err := os.Lstat(lp)
		if err != nil {
			t.Fatalf("lstat: %v", err)
		}
		if li.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s is a symlink, expected hardlink", lp)
		}
		bi, err := os.Stat(blockPath)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(li, bi) {
			t.Fatalf("%s is not the same inode as the block", lp)
		}
	}

	// fresh -> hardlink
	created, err := ensureStoreLink(blocksDir, heightsDir, 100, hash)
	if err != nil || !created {
		t.Fatalf("fresh: created=%v err=%v", created, err)
	}
	assertHardlink(t, linkPathFor(heightsDir, 100))

	// idempotent -> no change
	created, err = ensureStoreLink(blocksDir, heightsDir, 100, hash)
	if err != nil || created {
		t.Fatalf("idempotent: created=%v err=%v (want false)", created, err)
	}

	// pre-existing symlink -> repaired to hardlink
	lp := linkPathFor(heightsDir, 101)
	if err := os.Symlink("../"+hash.String()+".ods", lp); err != nil {
		t.Fatal(err)
	}
	created, err = ensureStoreLink(blocksDir, heightsDir, 101, hash)
	if err != nil || !created {
		t.Fatalf("repair: created=%v err=%v", created, err)
	}
	assertHardlink(t, lp)

	// empty EDS -> symlink with relative target
	empty := share.DataHash(share.EmptyEDSDataHash())
	created, err = ensureStoreLink(blocksDir, heightsDir, 102, empty)
	if err != nil || !created {
		t.Fatalf("empty: created=%v err=%v", created, err)
	}
	elp := linkPathFor(heightsDir, 102)
	fi, err := os.Lstat(elp)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("empty link is not a symlink: %v", err)
	}
	if got, _ := os.Readlink(elp); got != "../"+empty.String()+".ods" {
		t.Fatalf("empty target = %q", got)
	}
	// empty is idempotent too
	if created, err := ensureStoreLink(blocksDir, heightsDir, 102, empty); err != nil || created {
		t.Fatalf("empty idempotent: created=%v err=%v (want false)", created, err)
	}
}

// TestEmptyBlockReadableStaysSymlink pins the empty-EDS path: the store
// populates the canonical empty ODS, storeReadableHash reports it readable via
// OpenODS (no .q4 requirement), and ensureStoreLink keeps the height link a
// symlink — including repairing a hardlink that was wrongly placed there.
func TestEmptyBlockReadableStaysSymlink(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	// NewStore writes blocks/<emptyhash>.ods (+ .q4) via populateEmptyFile.
	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Stop(ctx)

	blocksDir := filepath.Join(base, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	empty := share.DataHash(share.EmptyEDSDataHash())

	// A height symlinked to the empty ODS must read as readable + empty.
	lp := linkPathFor(heightsDir, 500)
	if err := os.Symlink("../"+empty.String()+".ods", lp); err != nil {
		t.Fatal(err)
	}
	dh, ok := storeReadableHash(ctx, lp)
	if !ok {
		t.Fatal("empty block reported NOT readable via OpenODS")
	}
	if !dh.IsEmptyEDS() {
		t.Fatalf("expected empty datahash, got %s", dh)
	}

	// ensureStoreLink leaves a correct empty symlink untouched.
	if created, err := ensureStoreLink(blocksDir, heightsDir, 500, empty); err != nil || created {
		t.Fatalf("empty symlink should be left as-is: created=%v err=%v", created, err)
	}
	if fi, _ := os.Lstat(lp); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("empty height link is no longer a symlink")
	}

	// A hardlink wrongly placed at an empty height is repaired back to a symlink.
	hardLp := linkPathFor(heightsDir, 501)
	if err := os.Link(filepath.Join(blocksDir, empty.String()+".ods"), hardLp); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(hardLp); fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("setup: expected a hardlink")
	}
	created, err := ensureStoreLink(blocksDir, heightsDir, 501, empty)
	if err != nil || !created {
		t.Fatalf("empty hardlink repair: created=%v err=%v", created, err)
	}
	fi, err := os.Lstat(hardLp)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("empty height link was not converted to a symlink: %v", err)
	}
}
