package replicate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	log.Infow("verify: resolved window", "from", from, "to", to, "blocks_dir", blocksDir)

	var (
		checked, okCount, empty, absent, failed uint64
		startedAt                               = time.Now()
	)
	for h := from; h <= to; h++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		linkPath := filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
		li, err := os.Lstat(linkPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				absent++
				log.Debugw("verify: height absent", "height", h)
				continue
			}
			return fmt.Errorf("lstat height %d: %w", h, err)
		}

		checked++
		isEmpty, err := verifyHeight(ctx, blocksDir, linkPath, li)
		if err != nil {
			failed++
			log.Warnw("verify: FAILED", "height", h, "err", err)
			if cfg.FailFast {
				return fmt.Errorf("verify height %d: %w", h, err)
			}
			continue
		}
		okCount++
		if isEmpty {
			empty++
		}
		if checked%5000 == 0 {
			log.Infow("verify progress", "height", h,
				"checked", checked, "ok", okCount, "failed", failed,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}

	log.Infow("verify done",
		"checked", checked, "ok", okCount, "empty", empty, "absent", absent, "failed", failed,
		"elapsed", time.Since(startedAt).Round(time.Second))
	if failed > 0 {
		return fmt.Errorf("verification failed for %d of %d checked blocks", failed, checked)
	}
	return nil
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
