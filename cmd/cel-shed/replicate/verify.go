package replicate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store/file"
)

// VerifyConfig configures an offline, read-only integrity audit of a data
// directory's ODS files.
//
// For each present height in [from..to] verify checks three things:
//
//   - the DataHash stored in the ODS header matches the hash recomputed from the
//     shares actually on disk (the block's data is not silently corrupt);
//   - the backing blocks/<hash>.ods file carries that same hash — read from the
//     hardlinked file rather than assumed from the shared inode; and
//   - the height link obeys the store convention: a hardlink to the block for a
//     non-empty EDS, a symlink for the empty EDS.
//
// Nothing is modified and no network is used.
type VerifyConfig struct {
	DataDir    string
	FromHeight uint64
	ToHeight   uint64
	FailFast   bool
	// Concurrency is the number of parallel verification workers. 0 means one
	// worker per CPU core.
	Concurrency int
	// FailedFile is where failed heights (and their reasons) are written. Empty
	// means <data-dir>/.cel-shed-replicate/verify-failed.txt.
	FailedFile string
	LogLevel   string
}

func (c VerifyConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

func (c VerifyConfig) failedFilePath() string {
	if strings.TrimSpace(c.FailedFile) != "" {
		return c.FailedFile
	}
	return filepath.Join(c.DataDir, ".cel-shed-replicate", "verify-failed.txt")
}

// RunVerify scans [from..to] and audits every present height link and its
// backing block file. It returns an error if any block fails verification.
func RunVerify(ctx context.Context, cfg VerifyConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")

	log.Infow("verify: scanning heights dir", "dir", heightsDir)
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
	log.Infow("verify: resolved window", "from", from, "to", to, "blocks_dir", blocksDir, "workers", concurrency)

	// Each height is verified independently (own file handles, pure recompute),
	// so a worker pool fans the CPU-bound rebuild across cores. A shared context
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
				res := checkHeight(runCtx, blocksDir, heightsDir, h)
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

		// Progress is reported per completed height milestone, in ascending order
		// and without skipping any, despite workers finishing out of order. `done`
		// holds finished heights not yet contiguous with `watermark`; the watermark
		// is the highest height H with every height in [from..H] finished. It only
		// advances over a solid run, so a slow block holds back the milestone until
		// it (and everything below it) is done. `done` stays small: it never grows
		// past the in-flight window ahead of the watermark.
		done      = make(map[uint64]struct{})
		watermark = from
	)
	for res := range resultsCh {
		switch res.status {
		case statusAbsent:
			absent++
		case statusFatal:
			// An I/O error (not a verification failure) aborts the whole run.
			fatalErr = res.err
			cancel()
			continue
		case statusFailed:
			checked++
			failed++
			failedResults = append(failedResults, res)
			log.Warnw("verify: FAILED", "height", res.height, "err", res.err)
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

		// Advance the watermark over the now-contiguous run of finished heights
		// (absent heights count as finished, so real gaps don't stall it), logging
		// each milestone exactly once, in order, as it is crossed.
		done[res.height] = struct{}{}
		var milestones []uint64
		watermark, milestones = advanceWatermark(watermark, to, progressInterval, done)
		for _, m := range milestones {
			log.Infow("verify progress", "height", m,
				"checked", checked, "ok", okCount, "empty", empty, "absent", absent, "failed", failed,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}

	if fatalErr != nil {
		return fatalErr
	}

	// Results arrive out of order across workers; sort failures by height so the
	// failed file is stable and ascending (and cut-friendly for a repair pass).
	sort.Slice(failedResults, func(i, j int) bool { return failedResults[i].height < failedResults[j].height })
	failedLines := make([]string, len(failedResults))
	for i, res := range failedResults {
		failedLines[i] = fmt.Sprintf("%d\t%s", res.height, res.err)
	}

	failedPath := cfg.failedFilePath()
	if failed > 0 {
		if err := writeFailedFile(failedPath, failedLines); err != nil {
			log.Warnw("verify: could not write failed file", "path", failedPath, "err", err)
		} else {
			log.Infow("verify: wrote failed heights", "path", failedPath, "count", failed)
		}
	} else {
		// A clean run must not leave a stale failed file implying failures.
		if err := os.Remove(failedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warnw("verify: could not remove stale failed file", "path", failedPath, "err", err)
		}
	}

	log.Infow("verify done",
		"checked", checked, "ok", okCount, "empty", empty, "absent", absent, "failed", failed,
		"elapsed", time.Since(startedAt).Round(time.Second))
	if failed > 0 {
		return fmt.Errorf("verification failed for %d of %d checked blocks; see %s",
			failed, checked, failedPath)
	}
	return nil
}

// writeFailedFile writes the collected "<height>\t<reason>" lines to path,
// truncating it, creating the parent directory if needed.
func writeFailedFile(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for failed file: %w", err)
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write failed file: %w", err)
	}
	return nil
}

// progressInterval is the height spacing between verify progress log lines.
const progressInterval = 5000

// advanceWatermark consumes the contiguous run of finished heights starting at
// watermark from the done set (deleting them as it goes) and returns the new
// watermark together with every `interval` milestone height crossed, in
// ascending order. The watermark never moves past a height that has not
// finished, so milestones are emitted in order and none is skipped — regardless
// of the order results arrived. It stops at `to` to avoid overflowing past the
// range end.
func advanceWatermark(watermark, to, interval uint64, done map[uint64]struct{}) (uint64, []uint64) {
	var milestones []uint64
	for {
		if _, ok := done[watermark]; !ok {
			break
		}
		delete(done, watermark)
		if interval != 0 && watermark%interval == 0 {
			milestones = append(milestones, watermark)
		}
		if watermark == to {
			break
		}
		watermark++
	}
	return watermark, milestones
}

// verification outcome for a single height, passed from a worker to the collector.
const (
	statusOK     = iota // verified, non-empty block
	statusEmpty         // verified, empty EDS (symlink)
	statusAbsent        // no height link present (skipped, not a failure)
	statusFailed        // verification failed (corrupt data / bad link)
	statusFatal         // I/O error that aborts the whole run
)

type checkResult struct {
	height uint64
	status int
	err    error
}

// checkHeight verifies one height and classifies the outcome. It is safe to call
// concurrently: it only reads files and recomputes hashes, sharing no state.
func checkHeight(ctx context.Context, blocksDir, heightsDir string, h uint64) checkResult {
	linkPath := filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
	li, err := os.Lstat(linkPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return checkResult{height: h, status: statusAbsent}
		}
		return checkResult{height: h, status: statusFatal, err: fmt.Errorf("lstat height %d: %w", h, err)}
	}
	isEmpty, err := verifyHeight(ctx, blocksDir, linkPath, li)
	switch {
	case err != nil:
		return checkResult{height: h, status: statusFailed, err: err}
	case isEmpty:
		return checkResult{height: h, status: statusEmpty}
	default:
		return checkResult{height: h, status: statusOK}
	}
}

// verifyHeight audits a single height link and its backing block file. It
// reports whether the block is the empty EDS.
func verifyHeight(ctx context.Context, blocksDir, linkPath string, li os.FileInfo) (isEmpty bool, err error) {
	// 1. Read the DataHash the link file stores in its header and recompute it
	// from the shares on disk. A mismatch means the data is corrupt.
	hdrHash, dataHash, err := readAndComputeHash(ctx, linkPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(dataHash, hdrHash) {
		return false, fmt.Errorf("data corruption: shares hash to %s but header says %s", dataHash, hdrHash)
	}

	// 2. The empty EDS is symlinked to the shared canonical file; there is no
	// per-height block to compare against.
	if hdrHash.IsEmptyEDS() {
		if li.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("empty EDS height should be a symlink, found a regular file")
		}
		return true, nil
	}

	// 3. A non-empty EDS is hardlinked to its own blocks/<hash>.ods. Confirm the
	// backing file exists and carries the same hash.
	blockPath := filepath.Join(blocksDir, hdrHash.String()+".ods")
	bi, err := os.Stat(blockPath)
	if err != nil {
		return false, fmt.Errorf("backing block %s: %w", filepath.Base(blockPath), err)
	}

	// The height link and its block should share one inode, so the bytes — and
	// therefore the hash — are provably identical. If they are NOT the same inode
	// (broken hardlink, a stray copy, or a wrong symlink), fall back to reading
	// and recomputing the block's hash directly instead of trusting the inode.
	if os.SameFile(li, bi) {
		return false, nil
	}
	log.Warnw("verify: height link is not a hardlink to its block; comparing hashes directly",
		"link", linkPath, "block", blockPath)
	blkHdr, blkData, err := readAndComputeHash(ctx, blockPath)
	if err != nil {
		return false, fmt.Errorf("backing block %s: %w", filepath.Base(blockPath), err)
	}
	if !bytes.Equal(blkData, blkHdr) {
		return false, fmt.Errorf("backing block %s data corruption: shares hash to %s but header says %s",
			filepath.Base(blockPath), blkData, blkHdr)
	}
	if !bytes.Equal(blkHdr, hdrHash) {
		return false, fmt.Errorf("hash mismatch: height link is %s but backing block %s is %s",
			hdrHash, filepath.Base(blockPath), blkHdr)
	}
	return false, nil
}

// readAndComputeHash opens the ODS at path and returns the DataHash stored in
// its header alongside the DataHash independently recomputed from its shares.
func readAndComputeHash(ctx context.Context, path string) (header, computed share.DataHash, err error) {
	ods, err := file.OpenODS(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open ODS: %w", err)
	}
	defer func() { _ = ods.Close() }()

	header, err = ods.DataHash(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("read header hash: %w", err)
	}
	shares, err := ods.Shares(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("read shares: %w", err)
	}
	odsSize := int(math.Sqrt(float64(len(shares))))
	if odsSize*odsSize != len(shares) {
		return nil, nil, fmt.Errorf("share count %d is not a perfect square", len(shares))
	}
	_, _, computed, err = rebuild(shares, odsSize)
	if err != nil {
		return nil, nil, err
	}
	return header, computed, nil
}
