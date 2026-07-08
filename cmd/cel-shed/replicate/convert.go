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

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/store"
	"github.com/celestiaorg/celestia-node/store/file"
)

// ConvertConfig configures the in-place convert run.
//
// convert repairs a data directory OFFLINE and makes it match exactly what a
// node's store would have written. Two problems are fixed:
//
//   - blocks written by the old raw-ODS fetch contain the correct ODS shares
//     but not the store's ODSQ4 file format (header + sibling .q4). Their shares
//     are read from disk, the square is rebuilt, and the block is re-written via
//     store.PutODSQ4 — no network needed.
//   - height links created as symlinks by an earlier (buggy) tool are rewritten
//     as hardlinks, which is the store's convention for non-empty blocks (empty
//     EDS heights stay symlinks, per the store).
//
// Blocks already in store format with a correct hardlink are left untouched.
// Heights whose on-disk data is missing or corrupt are reported for
// `staged-sync --repair` (the network fallback).
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

// RunConvert scans [from..to] and, in place and offline, re-encodes every
// present-but-wrong-format block and normalises every height link to the store
// convention (hardlink for non-empty, symlink for empty).
func RunConvert(ctx context.Context, cfg ConvertConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")

	// Open the real EDS store: PutODSQ4 writes byte-identical files and the
	// store-convention hardlink, and NewStore populates the canonical empty
	// EDS file so empty-height symlinks resolve.
	st, err := store.NewStore(store.DefaultParameters(), cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = st.Stop(stopCtx)
	}()

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
		reencoded, relinked, alreadyOK, empty, absent, failed uint64
		startedAt                                             = time.Now()
	)
	for h := from; h <= to; h++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		linkPath := filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
		if _, err := os.Lstat(linkPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				absent++
				log.Debugw("convert: height absent (needs fetch)", "height", h)
				continue
			}
			return fmt.Errorf("lstat height %d: %w", h, err)
		}

		// Case 1: block files are already store-readable. Only the height link
		// might be wrong (a symlink where a hardlink is expected) — fix it.
		if dh, ok := storeReadableHash(ctx, blocksDir, linkPath); ok {
			created, err := ensureStoreLink(blocksDir, heightsDir, h, dh)
			if err != nil {
				failed++
				log.Warnw("convert: relink failed", "height", h, "err", err)
				continue
			}
			switch {
			case dh.IsEmptyEDS():
				empty++
			case created:
				relinked++
				log.Infow("convert: fixed height link to hardlink", "height", h, "hash", dh.String())
			default:
				alreadyOK++
			}
			continue
		}

		// Case 2: not store-readable — rebuild the square from the on-disk data
		// and rewrite the block via the store (which also hardlinks the height).
		square, roots, hash, err := reconstructFromLink(ctx, linkPath)
		if err != nil {
			failed++
			log.Warnw("convert: block cannot be rebuilt locally (needs fetch)", "height", h, "err", err)
			continue
		}
		// Clear the old raw file + any stale link so PutODSQ4 writes cleanly.
		_ = os.Remove(filepath.Join(blocksDir, hash.String()+".ods"))
		_ = os.Remove(filepath.Join(blocksDir, hash.String()+".q4"))
		_ = os.Remove(linkPath)
		if err := st.PutODSQ4(ctx, roots, h, square.ExtendedDataSquare); err != nil {
			failed++
			log.Warnw("convert: store put failed", "height", h, "err", err)
			continue
		}
		reencoded++
		log.Infow("convert: block re-encoded via store", "height", h, "hash", hash.String())
		if (reencoded+relinked)%5000 == 0 {
			log.Infow("convert progress", "height", h,
				"reencoded", reencoded, "relinked", relinked, "already_ok", alreadyOK,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}

	log.Infow("convert done",
		"reencoded", reencoded, "relinked", relinked, "already_ok", alreadyOK,
		"empty", empty, "absent", absent, "failed", failed,
		"elapsed", time.Since(startedAt).Round(time.Second))
	if absent > 0 || failed > 0 {
		log.Infow("some heights need network fetch; run staged-sync --repair for them",
			"absent", absent, "failed", failed)
	}
	return nil
}

// storeReadableHash reports the block's DataHash and true if the block behind
// linkPath opens as a store ODS with a present, non-empty sibling .q4 (empty
// EDS blocks count as readable — the node manages their file).
func storeReadableHash(ctx context.Context, blocksDir, linkPath string) (share.DataHash, bool) {
	ods, err := file.OpenODS(linkPath)
	if err != nil {
		return nil, false
	}
	dh, err := ods.DataHash(ctx)
	_ = ods.Close()
	if err != nil {
		return nil, false
	}
	if dh.IsEmptyEDS() {
		return dh, true
	}
	q4 := filepath.Join(blocksDir, dh.String()+".q4")
	if info, err := os.Stat(q4); err != nil || info.Size() == 0 {
		return dh, false
	}
	return dh, true
}

// reconstructFromLink rebuilds the EDS behind linkPath from its on-disk shares.
// It handles both the raw-ODS format (OpenODS fails → the bytes are raw shares)
// and a valid ODS file that is merely missing its .q4 (read shares via the
// accessor). Returns the square, its roots, and the computed DataHash.
func reconstructFromLink(
	ctx context.Context,
	linkPath string,
) (*eds.Rsmt2D, *share.AxisRoots, share.DataHash, error) {
	if ods, err := file.OpenODS(linkPath); err == nil {
		shares, err := ods.Shares(ctx)
		_ = ods.Close()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read shares from ODS: %w", err)
		}
		odsSize := int(math.Sqrt(float64(len(shares))))
		if odsSize*odsSize != len(shares) {
			return nil, nil, nil, fmt.Errorf("ODS share count %d is not a perfect square", len(shares))
		}
		return rebuild(shares, odsSize)
	}

	raw, err := os.ReadFile(linkPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read raw ODS: %w", err)
	}
	return reconstructODS(raw, nil)
}

// reconstructODS rebuilds an EDS from raw row-major ODS share bytes (the shrex
// response / old raw-ODS on-disk format), verifying the computed hash against
// expected when non-nil.
func reconstructODS(
	raw []byte,
	expected share.DataHash,
) (*eds.Rsmt2D, *share.AxisRoots, share.DataHash, error) {
	if len(raw) == 0 || len(raw)%libshare.ShareSize != 0 {
		return nil, nil, nil, fmt.Errorf("not a raw ODS: size %d not a multiple of share size", len(raw))
	}
	totalShares := len(raw) / libshare.ShareSize
	odsSize := int(math.Sqrt(float64(totalShares)))
	if odsSize*odsSize != totalShares {
		return nil, nil, nil, fmt.Errorf("not a raw ODS: %d shares is not a perfect square", totalShares)
	}
	shares, err := eds.ReadShares(bytes.NewReader(raw), libshare.ShareSize, odsSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read shares: %w", err)
	}
	square, roots, hash, err := rebuild(shares, odsSize)
	if err != nil {
		return nil, nil, nil, err
	}
	if expected != nil && !expected.IsEmptyEDS() && !bytes.Equal(hash, expected) {
		return nil, nil, nil, fmt.Errorf("hash mismatch: data hashes to %s but expected %s", hash, expected)
	}
	return square, roots, hash, nil
}

// rebuild extends the ODS shares into a full square and computes its roots/hash.
func rebuild(shares []libshare.Share, odsSize int) (*eds.Rsmt2D, *share.AxisRoots, share.DataHash, error) {
	square, err := eds.Rsmt2DFromShares(shares, odsSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rebuild square: %w", err)
	}
	roots, err := share.NewAxisRoots(square.ExtendedDataSquare)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("compute roots: %w", err)
	}
	return square, roots, share.DataHash(roots.Hash()), nil
}

// encodeRawSharesAsODSQ4 rebuilds an EDS from raw ODS shares and writes the
// store pair <hash>.ods + <hash>.q4 under blocksDir via file.CreateODSQ4 (the
// same writer store.PutODSQ4 uses). Used by the fetch paths, which create the
// height link themselves. Returns the computed DataHash.
func encodeRawSharesAsODSQ4(
	blocksDir string,
	raw []byte,
	expected share.DataHash,
	overwrite bool,
) (share.DataHash, error) {
	square, roots, hash, err := reconstructODS(raw, expected)
	if err != nil {
		return nil, err
	}
	odsPath := filepath.Join(blocksDir, hash.String()+".ods")
	q4Path := filepath.Join(blocksDir, hash.String()+".q4")
	if overwrite {
		_ = os.Remove(odsPath)
		_ = os.Remove(q4Path)
	}
	if err := file.CreateODSQ4(odsPath, q4Path, roots, square.ExtendedDataSquare); err != nil {
		return nil, fmt.Errorf("write odsq4: %w", err)
	}
	return hash, nil
}
