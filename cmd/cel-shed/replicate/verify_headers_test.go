package replicate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

// fakeLookup builds a headerLookup backed by an in-memory height->DataHash map,
// so the verification logic can be exercised without any network header
// download.
func fakeLookup(m map[uint64]share.DataHash) headerLookup {
	return func(_ context.Context, height uint64) (share.DataHash, error) {
		return m[height], nil
	}
}

// fakeLookupWithUnavailable serves hashes from m and returns errHeaderUnavailable
// for any height not in it — simulating a --source peer whose head sits below
// those heights, so their headers were never downloaded.
func fakeLookupWithUnavailable(m map[uint64]share.DataHash) headerLookup {
	return func(_ context.Context, height uint64) (share.DataHash, error) {
		h, ok := m[height]
		if !ok {
			return nil, errHeaderUnavailable
		}
		return h, nil
	}
}

func lstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	li, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return li
}

// TestVerifyHeightAgainstHeader exercises the per-height, no-reconstruction
// check against an injected header hash for every outcome: a matching non-empty
// block, an empty-EDS symlink, a header/ODS mismatch, an ODS-header-vs-filename
// mismatch, and a broken hardlink.
func TestVerifyHeightAgainstHeader(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Stop(ctx)

	blocksDir := filepath.Join(base, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	linkFor := func(h uint64) string {
		return filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
	}

	// A real store-written non-empty block whose height link is a hardlink to
	// blocks/<hash>.ods.
	hash10 := putBlock(t, st, 10, 4)

	// An empty-EDS height: a symlink to the canonical empty ODS.
	empty := share.DataHash(share.EmptyEDSDataHash())
	if _, err := ensureStoreLink(blocksDir, heightsDir, 11, empty); err != nil {
		t.Fatalf("empty link: %v", err)
	}

	// Case: matching non-empty block passes.
	if _, err := verifyHeightAgainstHeader(ctx, blocksDir, linkFor(10), lstat(t, linkFor(10)), hash10); err != nil {
		t.Fatalf("matching non-empty block should pass, got: %v", err)
	}

	// Case: empty-EDS symlink passes and is reported empty.
	isEmpty, err := verifyHeightAgainstHeader(ctx, blocksDir, linkFor(11), lstat(t, linkFor(11)), empty)
	if err != nil {
		t.Fatalf("empty-EDS symlink should pass, got: %v", err)
	}
	if !isEmpty {
		t.Fatal("empty-EDS height should be reported as empty")
	}

	// Case: header/ODS mismatch fails (chain hash differs from the ODS header).
	wrong := putBlock(t, st, 99, 4) // some other real hash to use as the chain hash
	if _, err := verifyHeightAgainstHeader(ctx, blocksDir, linkFor(10), lstat(t, linkFor(10)), wrong); err == nil {
		t.Fatal("header/ODS mismatch should fail")
	} else if !strings.Contains(err.Error(), "header DataHash") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}

	// Case: ODS-header-vs-filename mismatch. Copy a real block's ODS bytes into a
	// file stored under a DIFFERENT filename, then symlink a height at it. The ODS
	// header hash read from the content will not match the filename it is stored
	// under.
	misnamedHash := putBlock(t, st, 90, 8) // a valid, distinct hash to use as the wrong filename
	misnamedPath := filepath.Join(blocksDir, misnamedHash.String()+".ods")
	if err := os.Remove(misnamedPath); err != nil {
		t.Fatalf("remove misnamed block placeholder: %v", err)
	}
	// Content is block 10's ODS (header hash10), but the file is named misnamedHash.
	srcBytes, err := os.ReadFile(filepath.Join(blocksDir, hash10.String()+".ods"))
	if err != nil {
		t.Fatalf("read block 10 ODS: %v", err)
	}
	if err := os.WriteFile(misnamedPath, srcBytes, 0o644); err != nil {
		t.Fatalf("write misnamed block: %v", err)
	}
	_ = os.Remove(linkFor(12))
	if err := os.Symlink("../"+misnamedHash.String()+".ods", linkFor(12)); err != nil {
		t.Fatalf("symlink misnamed target: %v", err)
	}
	// check (1) passes (chain hash == ODS header hash10), check (2) — the link
	// target filename (misnamedHash) != ODS header hash (hash10) — fires.
	if _, err := verifyHeightAgainstHeader(ctx, blocksDir, linkFor(12), lstat(t, linkFor(12)), hash10); err == nil {
		t.Fatal("ODS-header-vs-filename mismatch should fail")
	} else if !strings.Contains(err.Error(), "block filename hash") {
		t.Fatalf("unexpected filename-mismatch error: %v", err)
	}

	// Case: broken hardlink. Remove the backing block file for height 10 so the
	// height link no longer resolves to blocks/<hash>.ods.
	// First restore a clean non-empty hardlinked height to break.
	hash13 := putBlock(t, st, 13, 4)
	if err := os.Remove(filepath.Join(blocksDir, hash13.String()+".ods")); err != nil {
		t.Fatalf("remove backing block: %v", err)
	}
	if _, err := verifyHeightAgainstHeader(ctx, blocksDir, linkFor(13), lstat(t, linkFor(13)), hash13); err == nil {
		t.Fatal("broken hardlink (missing backing block) should fail")
	} else if !strings.Contains(err.Error(), "block filename hash") &&
		!strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("unexpected broken-hardlink error: %v", err)
	}
}

// TestRunVerifyHeadersSkipsUnavailableHeaders proves that a present block whose
// header was not downloaded (it is above the source's head) is skipped and the
// run completes cleanly, rather than blocking or being reported as a failure.
func TestRunVerifyHeadersSkipsUnavailableHeaders(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Stop(ctx)

	// Three present blocks on disk, but the "source" only holds headers up to 11
	// (its head is 11); height 12 is above the source head.
	hash10 := putBlock(t, st, 10, 4)
	hash11 := putBlock(t, st, 11, 8)
	_ = putBlock(t, st, 12, 4)

	cfg := VerifyHeadersConfig{DataDir: base, FromHeight: 10, ToHeight: 12, LogLevel: "error"}
	lookup := fakeLookupWithUnavailable(map[uint64]share.DataHash{10: hash10, 11: hash11})

	// 10 and 11 verify; 12 is skipped. A skip is not a failure, so the run passes.
	if err := runVerifyHeaders(ctx, cfg, lookup); err != nil {
		t.Fatalf("unavailable header should be skipped and the run should pass, got: %v", err)
	}
	// Skipped (not failed) heights must not leave a failed file behind.
	if _, err := os.Stat(cfg.failedFilePath()); !os.IsNotExist(err) {
		t.Fatalf("skipped heights must not create a failed file, stat err = %v", err)
	}
}

// TestRunVerifyHeaders drives the full download-independent path (scan + worker
// pool + failed-file handling) with an injected in-memory header lookup, so no
// network is required.
func TestRunVerifyHeaders(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	st, err := store.NewStore(store.DefaultParameters(), base)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Stop(ctx)

	hash10 := putBlock(t, st, 10, 4)
	hash11 := putBlock(t, st, 11, 8)
	empty := share.DataHash(share.EmptyEDSDataHash())
	if _, err := ensureStoreLink(base+"/blocks", base+"/blocks/heights", 12, empty); err != nil {
		t.Fatalf("empty link: %v", err)
	}

	cfg := VerifyHeadersConfig{DataDir: base, FromHeight: 10, ToHeight: 12, LogLevel: "error"}
	good := fakeLookup(map[uint64]share.DataHash{10: hash10, 11: hash11, 12: empty})
	if err := runVerifyHeaders(ctx, cfg, good); err != nil {
		t.Fatalf("clean data should verify, got: %v", err)
	}

	// A wrong chain hash for height 11 must fail and be recorded in the failed file.
	bad := fakeLookup(map[uint64]share.DataHash{10: hash10, 11: hash10, 12: empty})
	if err := runVerifyHeaders(ctx, cfg, bad); err == nil {
		t.Fatal("expected verification to fail on a header/ODS mismatch")
	}
	failedPath := cfg.failedFilePath()
	data, err := os.ReadFile(failedPath)
	if err != nil {
		t.Fatalf("read failed file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "11\t") {
		t.Fatalf("failed file = %q, want a single line for height 11", string(data))
	}

	// A clean run over only intact heights should pass again and clear the stale
	// failed file.
	clean := VerifyHeadersConfig{DataDir: base, FromHeight: 10, ToHeight: 10, LogLevel: "error"}
	if err := runVerifyHeaders(ctx, clean, good); err != nil {
		t.Fatalf("intact height should verify, got: %v", err)
	}
	if _, err := os.Stat(failedPath); !os.IsNotExist(err) {
		t.Fatalf("clean run should remove stale failed file, stat err = %v", err)
	}
}
