package replicate

import (
	"bytes"
	"context"
	"encoding/hex"
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

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/store/file"
)

// ConvertConfig configures the in-place convert run.
//
// convert repairs a data directory OFFLINE: blocks written by the old raw-ODS
// fetch contain the correct ODS shares but not the store's ODSQ4 file format
// (header + sibling .q4). Because the share data is already on disk, there is no
// need to re-download anything — convert reads the raw shares, rebuilds the
// square, recomputes the DataHash locally, and rewrites each block as a proper
// <hash>.ods + <hash>.q4 pair, repointing the height link at it.
//
// Blocks that are already in store format (or empty-EDS heights) are skipped.
// Heights whose on-disk data is missing or corrupt are reported and left for
// `staged-sync --repair`, which can re-fetch them from the network.
type ConvertConfig struct {
	DataDir    string
	FromHeight uint64
	ToHeight   uint64
	LogLevel   string
}

func (c ConvertConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

// RunConvert scans [from..to] and re-encodes every present-but-wrong-format
// block in place, without touching the network.
func RunConvert(ctx context.Context, cfg ConvertConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	emptyHex := hex.EncodeToString(share.EmptyEDSDataHash())

	log.Infow("convert: scanning heights dir", "dir", heightsDir)
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
	log.Infow("convert: resolved window", "from", from, "to", to)

	var (
		converted, alreadyOK, empty, absent, failed uint64
		startedAt                                   = time.Now()
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
				log.Debugw("convert: height absent (needs fetch)", "height", h)
			} else {
				return fmt.Errorf("lstat height %d: %w", h, err)
			}
			continue
		}

		// Empty-EDS heights are symlinks to the node-managed empty ODS; leave them.
		if li.Mode()&os.ModeSymlink != 0 {
			if tgt, err := os.Readlink(linkPath); err == nil {
				base := strings.TrimSuffix(filepath.Base(tgt), ".ods")
				if strings.EqualFold(base, emptyHex) {
					empty++
					continue
				}
			}
		}

		// Already store-readable? Skip.
		if isStoreReadable(ctx, blocksDir, linkPath) {
			alreadyOK++
			continue
		}

		hash, err := convertOne(blocksDir, linkPath)
		if err != nil {
			failed++
			log.Warnw("convert: block cannot be converted locally (needs fetch)",
				"height", h, "err", err)
			continue
		}
		converted++
		log.Infow("convert: block re-encoded",
			"height", h, "hash", hash.String(),
			"converted_total", converted)
		if converted%5000 == 0 {
			log.Infow("convert progress", "height", h, "converted", converted,
				"already_ok", alreadyOK, "elapsed", time.Since(startedAt).Round(time.Second))
		}
	}

	log.Infow("convert done",
		"converted", converted, "already_ok", alreadyOK, "empty", empty,
		"absent", absent, "failed", failed,
		"elapsed", time.Since(startedAt).Round(time.Second))
	if absent > 0 || failed > 0 {
		log.Infow("some heights need network fetch; run staged-sync --repair for them",
			"absent", absent, "failed", failed)
	}
	return nil
}

// isStoreReadable reports whether the block behind linkPath opens as a store
// ODS with a present, non-empty sibling .q4.
func isStoreReadable(ctx context.Context, blocksDir, linkPath string) bool {
	ods, err := file.OpenODS(linkPath)
	if err != nil {
		return false
	}
	dh, err := ods.DataHash(ctx)
	_ = ods.Close()
	if err != nil {
		return false
	}
	q4 := filepath.Join(blocksDir, dh.String()+".q4")
	info, err := os.Stat(q4)
	return err == nil && info.Size() > 0
}

// convertOne reads the raw ODS shares behind linkPath, rebuilds the square,
// recomputes the roots/hash, and rewrites the block as a store ODSQ4 pair
// (<hash>.ods + <hash>.q4) with the height link repointed at it. Returns the
// computed DataHash. Errors if the on-disk data is not a valid raw ODS.
func convertOne(blocksDir, linkPath string) (share.DataHash, error) {
	// Read the data through the link (works for both hardlink and symlink).
	raw, err := os.ReadFile(linkPath)
	if err != nil {
		return nil, fmt.Errorf("read raw ODS: %w", err)
	}
	if len(raw) == 0 || len(raw)%libshare.ShareSize != 0 {
		return nil, fmt.Errorf("not a raw ODS: size %d not a multiple of share size", len(raw))
	}
	totalShares := len(raw) / libshare.ShareSize
	odsSize := int(math.Sqrt(float64(totalShares)))
	if odsSize*odsSize != totalShares {
		return nil, fmt.Errorf("not a raw ODS: %d shares is not a perfect square", totalShares)
	}

	shares, err := eds.ReadShares(bytes.NewReader(raw), libshare.ShareSize, odsSize)
	if err != nil {
		return nil, fmt.Errorf("read shares: %w", err)
	}
	square, err := eds.Rsmt2DFromShares(shares, odsSize)
	if err != nil {
		return nil, fmt.Errorf("rebuild square: %w", err)
	}
	roots, err := share.NewAxisRoots(square.ExtendedDataSquare)
	if err != nil {
		return nil, fmt.Errorf("compute roots: %w", err)
	}
	hash := share.DataHash(roots.Hash())

	// If the link is a symlink naming a hash, sanity-check it matches.
	if tgt, err := os.Readlink(linkPath); err == nil {
		named := strings.TrimSuffix(filepath.Base(tgt), ".ods")
		if named != "" && !strings.EqualFold(named, hex.EncodeToString(hash)) {
			return nil, fmt.Errorf("hash mismatch: link names %s but data hashes to %s", named, hash)
		}
	}

	odsPath := filepath.Join(blocksDir, hash.String()+".ods")
	q4Path := filepath.Join(blocksDir, hash.String()+".q4")

	// Remove the old raw file(s) before writing (CreateODS uses O_EXCL). The raw
	// bytes are already in memory, and the height hardlink (if any) keeps the
	// inode alive until we repoint it below.
	_ = os.Remove(odsPath)
	_ = os.Remove(q4Path)
	if err := file.CreateODSQ4(odsPath, q4Path, roots, square.ExtendedDataSquare); err != nil {
		return nil, fmt.Errorf("write odsq4: %w", err)
	}

	// Repoint the height link at the freshly written ODS as a symlink.
	if _, err := ensureSymlink(linkPath, "../"+hash.String()+".ods"); err != nil {
		return nil, fmt.Errorf("relink height: %w", err)
	}
	return hash, nil
}
