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

	// heights file present but blocks/<hash>.ods missing -> the block name is
	// hardlinked to the heights file; the data (the only copy) must survive.
	orphan := make([]byte, share.DataHashSize)
	for i := range orphan {
		orphan[i] = byte(i + 3)
	}
	orphanHash := share.DataHash(orphan)
	olp := linkPathFor(heightsDir, 103)
	if err := os.WriteFile(olp, []byte("ONLYCOPY"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = ensureStoreLink(blocksDir, heightsDir, 103, orphanHash)
	if err != nil || !created {
		t.Fatalf("orphan heights file: created=%v err=%v", created, err)
	}
	orphanBlock := filepath.Join(blocksDir, orphanHash.String()+".ods")
	li, err := os.Lstat(olp)
	if err != nil {
		t.Fatalf("heights file vanished: %v", err)
	}
	bi, err := os.Stat(orphanBlock)
	if err != nil {
		t.Fatalf("blocks/<hash>.ods was not created: %v", err)
	}
	if !os.SameFile(li, bi) {
		t.Fatal("blocks/<hash>.ods is not a hardlink to the heights file")
	}
	if data, _ := os.ReadFile(orphanBlock); string(data) != "ONLYCOPY" {
		t.Fatalf("block content = %q, data lost", data)
	}
	// and it is idempotent afterwards
	if created, err := ensureStoreLink(blocksDir, heightsDir, 103, orphanHash); err != nil || created {
		t.Fatalf("orphan idempotent: created=%v err=%v (want false)", created, err)
	}

	// A separate heights copy must NOT be dropped for an unreadable blocks file
	// ("BLOCKDATA" does not parse as an ODS): error out, both files untouched.
	dupLp := linkPathFor(heightsDir, 104)
	if err := os.WriteFile(dupLp, []byte("HEIGHTCOPY"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureStoreLink(blocksDir, heightsDir, 104, hash); err == nil {
		t.Fatal("expected error for unreadable blocks file, got nil")
	}
	if data, _ := os.ReadFile(dupLp); string(data) != "HEIGHTCOPY" {
		t.Fatalf("heights copy was modified: %q", data)
	}
	if data, _ := os.ReadFile(blockPath); string(data) != "BLOCKDATA" {
		t.Fatalf("blocks file was modified: %q", data)
	}

	// A dangling symlink with no blocks file: error out, symlink left in place.
	danglingLp := linkPathFor(heightsDir, 105)
	if err := os.Symlink("../does-not-exist.ods", danglingLp); err != nil {
		t.Fatal(err)
	}
	missing := make([]byte, share.DataHashSize)
	for i := range missing {
		missing[i] = byte(i + 7)
	}
	if _, err := ensureStoreLink(blocksDir, heightsDir, 105, share.DataHash(missing)); err == nil {
		t.Fatal("expected error for dangling symlink, got nil")
	}
	if fi, err := os.Lstat(danglingLp); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dangling symlink was removed or replaced: %v", err)
	}

	// Empty height held as a regular file while the canonical empty ODS is
	// missing: the copy must first receive the canonical name, then become a
	// symlink that resolves to it — no bytes lost.
	emptyCopyLp := linkPathFor(heightsDir, 106)
	if err := os.WriteFile(emptyCopyLp, []byte("EMPTYCOPY"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = ensureStoreLink(blocksDir, heightsDir, 106, empty)
	if err != nil || !created {
		t.Fatalf("empty copy: created=%v err=%v", created, err)
	}
	if fi, err := os.Lstat(emptyCopyLp); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("empty height is not a symlink: %v", err)
	}
	if data, err := os.ReadFile(emptyCopyLp); err != nil || string(data) != "EMPTYCOPY" {
		t.Fatalf("empty symlink does not resolve to the preserved copy: %q err=%v", data, err)
	}
}

// TestRunConvertLinkOnly pins --link-only against the "orphaned heights file"
// state: heights/<h>.ods is a store-format regular file but blocks/<hash>.ods
// (and .q4) are gone. The run must restore the blocks hardlink onto the same
// inode without writing a .q4 or re-encoding, and must leave a
// non-store-readable block untouched.
func TestRunConvertLinkOnly(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	hash := putBlock(t, st, 700, 4)
	if err := st.Stop(ctx); err != nil {
		t.Fatalf("stop store: %v", err)
	}

	blocksDir := filepath.Join(base, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	blockPath := filepath.Join(blocksDir, hash.String()+".ods")
	q4Path := filepath.Join(blocksDir, hash.String()+".q4")

	// Orphan the heights file: drop the blocks-side names, keep the inode
	// alive through heights/700.ods.
	if err := os.Remove(blockPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(q4Path); err != nil {
		t.Fatal(err)
	}

	// A raw (non-store-readable) block that link-only must not touch.
	rawLp := linkPathFor(heightsDir, 701)
	if err := os.WriteFile(rawLp, []byte("not an ods"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A broken hardlink: a byte-identical separate copy of the same block at
	// another height (as left by rsync without -H). Must end up on one inode.
	odsBytes, err := os.ReadFile(linkPathFor(heightsDir, 700))
	if err != nil {
		t.Fatal(err)
	}
	dupLp := linkPathFor(heightsDir, 702)
	if err := os.WriteFile(dupLp, odsBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	err = RunConvert(ctx, ConvertConfig{DataDir: base, LinkOnly: true})
	if err != nil {
		t.Fatalf("run convert: %v", err)
	}

	li, err := os.Lstat(linkPathFor(heightsDir, 700))
	if err != nil {
		t.Fatalf("heights file vanished: %v", err)
	}
	bi, err := os.Stat(blockPath)
	if err != nil {
		t.Fatalf("blocks/<hash>.ods was not restored: %v", err)
	}
	if !os.SameFile(li, bi) {
		t.Fatal("heights and blocks names are not one inode")
	}
	if _, err := os.Stat(q4Path); !os.IsNotExist(err) {
		t.Fatalf(".q4 was written in --link-only mode: %v", err)
	}
	if data, _ := os.ReadFile(rawLp); string(data) != "not an ods" {
		t.Fatalf("raw block was modified: %q", data)
	}
	di, err := os.Lstat(dupLp)
	if err != nil {
		t.Fatalf("duplicate-copy height vanished: %v", err)
	}
	if !os.SameFile(di, bi) {
		t.Fatal("duplicate-copy height was not merged onto the block's inode")
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
