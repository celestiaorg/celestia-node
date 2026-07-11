package replicate

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

// TestRunBothChecks exercises the verify-all orchestration (sequential and
// parallel) end to end over a real store-written data dir, with the header check
// driven by an injected in-memory lookup so no network download is required. The
// ODS check runs for real against the on-disk shares.
func TestRunBothChecks(t *testing.T) {
	ctx := context.Background()

	newFixture := func(t *testing.T) (base string, hashes map[uint64]share.DataHash) {
		t.Helper()
		base = t.TempDir()
		st, err := store.NewStore(store.DefaultParameters(), base)
		if err != nil {
			t.Fatalf("new store: %v", err)
		}
		defer st.Stop(ctx)

		hashes = map[uint64]share.DataHash{
			10: putBlock(t, st, 10, 4),
			11: putBlock(t, st, 11, 8),
		}
		empty := share.DataHash(share.EmptyEDSDataHash())
		if _, err := ensureStoreLink(base+"/blocks", base+"/blocks/heights", 12, empty); err != nil {
			t.Fatalf("empty link: %v", err)
		}
		hashes[12] = empty
		return base, hashes
	}

	// odsRunner / hdrRunner build the two injected check functions the same way
	// RunVerifyAll does, except the header check uses a fake lookup.
	odsRunner := func(base string) func(context.Context) error {
		cfg := VerifyConfig{DataDir: base, FromHeight: 10, ToHeight: 12, LogLevel: "error"}
		return func(ctx context.Context) error { return RunVerify(ctx, cfg) }
	}
	hdrRunner := func(base string, lookup headerLookup) func(context.Context) error {
		cfg := VerifyHeadersConfig{DataDir: base, FromHeight: 10, ToHeight: 12, LogLevel: "error"}
		return func(ctx context.Context) error { return runVerifyHeaders(ctx, cfg, lookup) }
	}

	for _, parallel := range []bool{false, true} {
		t.Run(mode(parallel)+"/clean", func(t *testing.T) {
			base, hashes := newFixture(t)
			good := fakeLookup(hashes)
			if err := runBothChecks(ctx, parallel, odsRunner(base), hdrRunner(base, good)); err != nil {
				t.Fatalf("clean data should pass both checks, got: %v", err)
			}
		})

		t.Run(mode(parallel)+"/ods_only_failure", func(t *testing.T) {
			// Corrupt a share byte: the ODS check recomputes from shares and fails,
			// but the stored header hash is unchanged so the (fast) header check still
			// passes — proving the two checks are complementary and both run.
			base, hashes := newFixture(t)
			corruptLastByte(t, base+"/blocks/"+hashes[11].String()+".ods")
			err := runBothChecks(ctx, parallel, odsRunner(base), hdrRunner(base, fakeLookup(hashes)))
			if err == nil {
				t.Fatal("corrupted shares should fail verify-all via the ODS check")
			}
			if _, statErr := os.Stat(VerifyConfig{DataDir: base}.failedFilePath()); statErr != nil {
				t.Fatalf("ODS failed file should exist: %v", statErr)
			}
		})

		t.Run(mode(parallel)+"/headers_only_failure", func(t *testing.T) {
			// Clean data, but the chain lookup returns the wrong hash for height 11:
			// the header check fails while the ODS check passes.
			base, hashes := newFixture(t)
			bad := fakeLookup(map[uint64]share.DataHash{10: hashes[10], 11: hashes[10], 12: hashes[12]})
			err := runBothChecks(ctx, parallel, odsRunner(base), hdrRunner(base, bad))
			if err == nil {
				t.Fatal("wrong chain hash should fail verify-all via the header check")
			}
			if _, statErr := os.Stat(VerifyHeadersConfig{DataDir: base}.failedFilePath()); statErr != nil {
				t.Fatalf("headers failed file should exist: %v", statErr)
			}
		})
	}
}

// TestRunBothChecksReportsBothFailures confirms that when both checks fail the
// returned error joins both, rather than surfacing only the first.
func TestRunBothChecksReportsBothFailures(t *testing.T) {
	ctx := context.Background()
	errODS := errors.New("ods boom")
	errHDR := errors.New("headers boom")

	err := runBothChecks(ctx, true,
		func(context.Context) error { return errODS },
		func(context.Context) error { return errHDR },
	)
	if !errors.Is(err, errODS) || !errors.Is(err, errHDR) {
		t.Fatalf("combined error should wrap both, got: %v", err)
	}
}

func mode(parallel bool) string {
	if parallel {
		return "parallel"
	}
	return "sequential"
}
