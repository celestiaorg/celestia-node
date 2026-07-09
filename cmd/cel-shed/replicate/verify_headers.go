package replicate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dsbadger "github.com/ipfs/go-ds-badger4"
	logging "github.com/ipfs/go-log/v2"

	libheadstore "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	"github.com/celestiaorg/celestia-node/header"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store/file"
)

// VerifyHeadersConfig configures a fast, NO-RECONSTRUCTION integrity audit of a
// data directory's ODS files against the chain-committed DataHash of every
// height's downloaded header.
//
// Unlike verify-ods, which recomputes each block's hash by re-extending the
// square (CPU-bound erasure reconstruction), verify-headers reads only the
// ~65-byte ODS header. For each present height in [from..to] it checks:
//
//   - the DataHash stored in the ODS header equals the DataHash committed in
//     that height's chain header (the main "hash against the headers" check);
//   - the ODS-header DataHash matches the blocks/<hash>.ods filename the block
//     is stored under (the hash "inside" the ODS is self-consistent); and
//   - the height link resolves to that backing block file per the store
//     convention: a hardlink (shared inode) to the block for a non-empty EDS,
//     a symlink for the empty EDS.
//
// Because it never reads or re-hashes the shares, it deliberately does NOT catch
// a flipped byte inside a share that leaves the stored ODS-header hash unchanged
// — use verify-ods for that. Nothing is modified.
//
// Headers are downloaded first (via the replicate headers package, into the
// node's header store under DataDir) unless a headerLookup is injected directly
// (used by tests). The DataHash to compare against is the header's committed
// DataHash (equal, by header validation, to DAH.Hash()).
type VerifyHeadersConfig struct {
	DataDir string
	// Source is the libp2p multiaddr of the bridge node headers are downloaded
	// from; required unless a headerLookup is injected.
	Source     string
	Network    modp2p.Network
	FromHeight uint64
	ToHeight   uint64
	FailFast   bool
	// Concurrency is the number of parallel verification workers. 0 means one
	// worker per CPU core.
	Concurrency int
	// HeaderConcurrency is the number of concurrent header range requests during
	// the download phase (1..32). 0 means 8.
	HeaderConcurrency int
	RequestTimeout    time.Duration
	// HeaderStoreDir is the standalone badger directory the downloaded headers
	// are written to; the node's own store under DataDir is never touched. Empty
	// means <data-dir>/.cel-shed-replicate/verify-headers-db.
	HeaderStoreDir string
	// FailedFile is where failed heights (and their reasons) are written. Empty
	// means <data-dir>/.cel-shed-replicate/verify-headers-failed.txt.
	FailedFile string
	LogLevel   string
}

func (c VerifyHeadersConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

func (c VerifyHeadersConfig) failedFilePath() string {
	if strings.TrimSpace(c.FailedFile) != "" {
		return c.FailedFile
	}
	return filepath.Join(c.DataDir, ".cel-shed-replicate", "verify-headers-failed.txt")
}

// headerLookup resolves a height to its chain-committed DataHash. It abstracts
// the header source so the verification logic can be unit-tested with an
// in-memory map, independent of the network download.
type headerLookup func(ctx context.Context, height uint64) (share.DataHash, error)

// RunVerifyHeaders downloads the headers for [from..to], then audits every
// present height link and its backing block against the chain-committed
// DataHash without any erasure reconstruction. It returns an error if any block
// fails verification.
func RunVerifyHeaders(ctx context.Context, cfg VerifyHeadersConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
		_ = logging.SetLogLevel("cmd-shed/replicate/headers", cfg.LogLevel)
	}

	// Download the headers into the node's header store, then read them back per
	// height. This mirrors the main replicate flow's header phase.
	lookup, closeLookup, err := downloadHeaderLookup(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeLookup()

	return runVerifyHeaders(ctx, cfg, lookup)
}

// downloadHeaderLookup runs the header download phase and returns a headerLookup
// backed by the node's on-disk header store, plus a closer.
func downloadHeaderLookup(ctx context.Context, cfg VerifyHeadersConfig) (headerLookup, func(), error) {
	if strings.TrimSpace(cfg.Source) == "" {
		return nil, nil, fmt.Errorf("source multiaddr is required")
	}
	headerConc := cfg.HeaderConcurrency
	if headerConc <= 0 {
		headerConc = 8
	}
	reqTimeout := cfg.RequestTimeout
	if reqTimeout <= 0 {
		reqTimeout = 30 * time.Second
	}
	storeDir := strings.TrimSpace(cfg.HeaderStoreDir)
	if storeDir == "" {
		storeDir = filepath.Join(cfg.DataDir, ".cel-shed-replicate", "verify-headers-db")
	}

	log.Infow("verify-headers: starting header download phase",
		"data_dir", cfg.DataDir, "header_store", storeDir, "from", cfg.FromHeight, "to", cfg.ToHeight)
	dlStart := time.Now()
	prog := headers.NewProgress()
	if err := headers.Run(ctx, headers.Config{
		Source:         cfg.Source,
		DataDir:        cfg.DataDir,
		StoreDir:       storeDir,
		Network:        cfg.Network,
		FromHeight:     cfg.FromHeight,
		ToHeight:       cfg.ToHeight,
		Concurrency:    headerConc,
		RequestTimeout: reqTimeout,
	}, prog); err != nil {
		return nil, nil, fmt.Errorf("download headers: %w", err)
	}
	log.Infow("verify-headers: header download done",
		"stored", prog.Stored(), "elapsed", time.Since(dlStart).Round(time.Second))

	// Read the headers back from the same standalone store the download wrote to,
	// leaving the node's own header store under DataDir untouched.
	hstore, closeStore, err := openStandaloneHeaderStore(ctx, storeDir)
	if err != nil {
		return nil, nil, err
	}
	lookup := headerStoreLookup(hstore)
	return lookup, closeStore, nil
}

// openStandaloneHeaderStore opens the badger datastore at dir and wraps it in a
// go-header store for height lookups. It is the read-back counterpart to the
// standalone store the download phase writes to, so the node's own store is
// never opened.
func openStandaloneHeaderStore(ctx context.Context, dir string) (
	*libheadstore.Store[*header.ExtendedHeader],
	func(),
	error,
) {
	db, err := dsbadger.NewDatastore(dir, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open header db %q: %w", dir, err)
	}
	hstore, err := libheadstore.NewStore[*header.ExtendedHeader](db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("new header store: %w", err)
	}
	if err := hstore.Start(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("start header store: %w", err)
	}
	closeFn := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := hstore.Stop(stopCtx); err != nil {
			log.Warnw("verify-headers: stop header store", "err", err)
		}
		if err := db.Close(); err != nil {
			log.Warnw("verify-headers: close header db", "err", err)
		}
	}
	return hstore, closeFn, nil
}

// headerStoreLookup adapts a go-header store to the headerLookup signature,
// returning each height's chain-committed DataHash.
func headerStoreLookup(hstore *libheadstore.Store[*header.ExtendedHeader]) headerLookup {
	return func(ctx context.Context, height uint64) (share.DataHash, error) {
		hdr, err := hstore.GetByHeight(ctx, height)
		if err != nil {
			return nil, err
		}
		return chainDataHash(hdr), nil
	}
}

// chainDataHash returns the DataHash committed in a header's chain (raw) header.
// It is validated equal to DAH.Hash() when the ExtendedHeader is constructed, so
// either could be used; the committed DataHash is the on-chain value we audit
// the stored ODS against.
func chainDataHash(hdr *header.ExtendedHeader) share.DataHash {
	return share.DataHash(hdr.DataHash)
}

// runVerifyHeaders is the download-independent core: it scans [from..to] and
// audits each present height against the injected header lookup.
func runVerifyHeaders(ctx context.Context, cfg VerifyHeadersConfig, lookup headerLookup) error {
	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")

	log.Infow("verify-headers: scanning heights dir", "dir", heightsDir)
	present, err := scanHeightsDir(heightsDir)
	if err != nil {
		return fmt.Errorf("scan heights dir: %w", err)
	}
	sort.Slice(present, func(i, j int) bool { return present[i] < present[j] })

	from, to := cfg.FromHeight, cfg.ToHeight
	if from == 0 || to == 0 {
		if len(present) == 0 {
			return fmt.Errorf("heights dir is empty and no --from-height/--to-height given")
		}
		if from == 0 {
			from = present[0]
		}
		if to == 0 {
			to = present[len(present)-1]
		}
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	log.Infow("verify-headers: resolved window",
		"from", from, "to", to, "blocks_dir", blocksDir, "workers", concurrency)

	// Each height is audited independently (own file handles, header-only reads),
	// so a worker pool fans the I/O-bound checks across cores. A shared context
	// lets fail-fast / fatal errors stop the producer and remaining workers.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	heightsCh := make(chan uint64, concurrency*2)
	resultsCh := make(chan checkResult, concurrency*2)

	go func() {
		defer close(heightsCh)
		for h := from; h <= to; h++ {
			select {
			case <-runCtx.Done():
				return
			case heightsCh <- h:
			}
		}
	}()

	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for h := range heightsCh {
				res := checkHeightAgainstHeader(runCtx, blocksDir, heightsDir, h, lookup)
				select {
				case <-runCtx.Done():
					return
				case resultsCh <- res:
				}
			}
		}()
	}
	go func() { wg.Wait(); close(resultsCh) }()

	var (
		checked, okCount, empty, absent, failed uint64
		failedResults                           []checkResult
		fatalErr                                error
		startedAt                               = time.Now()

		// See RunVerify: the watermark advances only over a contiguous run of
		// finished heights so milestones are logged in order and none is skipped
		// despite workers finishing out of order.
		done      = make(map[uint64]struct{})
		watermark = from
	)
	for res := range resultsCh {
		switch res.status {
		case statusAbsent:
			absent++
		case statusFatal:
			fatalErr = res.err
			cancel()
			continue
		case statusFailed:
			checked++
			failed++
			failedResults = append(failedResults, res)
			log.Warnw("verify-headers: FAILED", "height", res.height, "err", res.err)
			if cfg.FailFast {
				cancel()
			}
		case statusEmpty:
			checked++
			okCount++
			empty++
		case statusOK:
			checked++
			okCount++
		}

		done[res.height] = struct{}{}
		var milestones []uint64
		watermark, milestones = advanceWatermark(watermark, to, progressInterval, done)
		for _, m := range milestones {
			log.Infow("verify-headers progress", "height", m,
				"checked", checked, "ok", okCount, "empty", empty, "absent", absent, "failed", failed,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}

	if fatalErr != nil {
		return fatalErr
	}

	sort.Slice(failedResults, func(i, j int) bool { return failedResults[i].height < failedResults[j].height })
	failedLines := make([]string, len(failedResults))
	for i, res := range failedResults {
		failedLines[i] = fmt.Sprintf("%d\t%s", res.height, res.err)
	}

	failedPath := cfg.failedFilePath()
	if failed > 0 {
		if err := writeFailedFile(failedPath, failedLines); err != nil {
			log.Warnw("verify-headers: could not write failed file", "path", failedPath, "err", err)
		} else {
			log.Infow("verify-headers: wrote failed heights", "path", failedPath, "count", failed)
		}
	} else {
		if err := os.Remove(failedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warnw("verify-headers: could not remove stale failed file", "path", failedPath, "err", err)
		}
	}

	log.Infow("verify-headers done",
		"checked", checked, "ok", okCount, "empty", empty, "absent", absent, "failed", failed,
		"elapsed", time.Since(startedAt).Round(time.Second))
	if failed > 0 {
		return fmt.Errorf("verification failed for %d of %d checked blocks; see %s",
			failed, checked, failedPath)
	}
	return nil
}

// checkHeightAgainstHeader audits one height against its chain header and
// classifies the outcome. It is safe to call concurrently: it only reads files
// and the (concurrency-safe) header store, sharing no state.
func checkHeightAgainstHeader(
	ctx context.Context,
	blocksDir, heightsDir string,
	h uint64,
	lookup headerLookup,
) checkResult {
	linkPath := filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
	li, err := os.Lstat(linkPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return checkResult{height: h, status: statusAbsent}
		}
		return checkResult{height: h, status: statusFatal, err: fmt.Errorf("lstat height %d: %w", h, err)}
	}

	chainHash, err := lookup(ctx, h)
	if err != nil {
		return checkResult{height: h, status: statusFatal, err: fmt.Errorf("lookup header %d: %w", h, err)}
	}

	isEmpty, err := verifyHeightAgainstHeader(ctx, blocksDir, linkPath, li, chainHash)
	switch {
	case err != nil:
		return checkResult{height: h, status: statusFailed, err: err}
	case isEmpty:
		return checkResult{height: h, status: statusEmpty}
	default:
		return checkResult{height: h, status: statusOK}
	}
}

// verifyHeightAgainstHeader audits a single height link and its backing block
// against the chain-committed DataHash, reading only ODS headers (no shares, no
// reconstruction). It reports whether the block is the empty EDS.
func verifyHeightAgainstHeader(
	ctx context.Context,
	blocksDir, linkPath string,
	li os.FileInfo,
	chainHash share.DataHash,
) (isEmpty bool, err error) {
	// 1. Read the DataHash the height link stores in its ODS header (header-only
	// read: ~65 bytes, no shares). Compare it to the chain header's DataHash —
	// the main "hash against the headers" check.
	odsHash, err := odsHeaderHash(ctx, linkPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(odsHash, chainHash) {
		return false, fmt.Errorf("header DataHash %s != ODS DataHash %s", chainHash, odsHash)
	}

	// 2. The empty EDS is symlinked to the shared canonical file; there is no
	// per-height block filename to compare against.
	if odsHash.IsEmptyEDS() {
		if li.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("empty EDS height should be a symlink, found a regular file")
		}
		return true, nil
	}

	// 3. A non-empty EDS is stored as blocks/<hash>.ods and the height link
	// hardlinks to it. The filename encodes the hash, so the block file named
	// after the ODS header hash must both exist and be the very inode the height
	// link resolves to. This proves two things at once: the hash inside the ODS
	// matches the block filename it is stored under, and the height link genuinely
	// resolves to blocks/<hash>.ods.
	blockPath := filepath.Join(blocksDir, odsHash.String()+".ods")

	// If the height link is itself a symlink (older layout), its target filename
	// is authoritative for "the block filename it is stored under": compare it to
	// the ODS header hash directly for a precise mismatch reason.
	if li.Mode()&os.ModeSymlink != 0 {
		if tgt, rerr := os.Readlink(linkPath); rerr == nil {
			base := strings.TrimSuffix(filepath.Base(tgt), ".ods")
			if !strings.EqualFold(base, odsHash.String()) {
				return false, fmt.Errorf("ODS header hash != block filename hash: header says %s but link points to %s",
					odsHash, filepath.Base(tgt))
			}
		}
	}

	bi, err := os.Stat(blockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("ODS header hash != block filename hash: no block file blocks/%s.ods for header hash %s",
				odsHash.String(), odsHash)
		}
		return false, fmt.Errorf("backing block %s: %w", filepath.Base(blockPath), err)
	}
	if !os.SameFile(li, bi) {
		return false, fmt.Errorf("height link does not resolve to blocks/%s.ods (not the same inode)",
			odsHash.String())
	}
	return false, nil
}

// odsHeaderHash opens the ODS at path and returns only the DataHash stored in
// its header. It reads the file header (~65 bytes) and does NOT read shares or
// rebuild the square.
func odsHeaderHash(ctx context.Context, path string) (share.DataHash, error) {
	ods, err := file.OpenODS(path)
	if err != nil {
		return nil, fmt.Errorf("open ODS: %w", err)
	}
	defer func() { _ = ods.Close() }()

	dh, err := ods.DataHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("read header hash: %w", err)
	}
	return dh, nil
}
