package replicate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"

	libhead "github.com/celestiaorg/go-header"

	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// GetMissingConfig configures the get-missing scan.
type GetMissingConfig struct {
	DataDir    string
	Network    modp2p.Network
	FromHeight uint64
	ToHeight   uint64
	LogLevel   string
}

func (c GetMissingConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

func (c GetMissingConfig) workdir() string {
	return filepath.Join(c.DataDir, ".cel-shed-replicate")
}

// RunGetMissing scans the heights/ directory for gaps, then reconciles each gap
// against the header store + blocks/ directory: if the on-disk ODS exists for
// the expected hash, it publishes the height link; otherwise it appends the
// height to missing.txt for later fetching.
func RunGetMissing(ctx context.Context, cfg GetMissingConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	heightsDir := filepath.Join(cfg.DataDir, "blocks", "heights")
	log.Infow("scanning heights dir", "dir", heightsDir)
	scanStart := time.Now()
	present, err := scanHeightsDir(heightsDir)
	if err != nil {
		return fmt.Errorf("scan heights dir: %w", err)
	}
	sort.Slice(present, func(i, j int) bool { return present[i] < present[j] })
	var presentMin, presentMax uint64
	if len(present) > 0 {
		presentMin, presentMax = present[0], present[len(present)-1]
	}
	log.Infow("heights dir scanned",
		"present", len(present),
		"min", presentMin,
		"max", presentMax,
		"elapsed", time.Since(scanStart).Round(time.Millisecond),
	)

	if err := os.MkdirAll(cfg.workdir(), 0o755); err != nil {
		return fmt.Errorf("ensure workdir: %w", err)
	}
	presentFile := filepath.Join(cfg.workdir(), "present.txt")
	if err := writeSortedHeights(presentFile, present); err != nil {
		return fmt.Errorf("write present.txt: %w", err)
	}
	log.Infow("wrote present heights", "file", presentFile, "count", len(present))

	log.Infow("opening header store")
	openStart := time.Now()
	hstore, closeStore, err := openHeaderStore(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer closeStore()
	log.Infow("opened header store", "elapsed", time.Since(openStart).Round(time.Second))

	head, err := hstore.Head(ctx)
	if err != nil {
		if errors.Is(err, libhead.ErrEmptyStore) {
			return fmt.Errorf("header store is empty")
		}
		return fmt.Errorf("read header head: %w", err)
	}

	from := cfg.FromHeight
	if from == 0 {
		from = 1
	}
	to := cfg.ToHeight
	if to == 0 {
		to = head.Height()
	}
	if to > head.Height() {
		return fmt.Errorf("to-height %d is above local header head %d", to, head.Height())
	}
	if from > to {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", from, to)
	}

	gaps := computeGaps(present, from, to)
	log.Infow("gap range resolved", "from", from, "to", to, "gaps", len(gaps))

	gapsFile := filepath.Join(cfg.workdir(), "gaps.txt")
	if err := writeSortedHeights(gapsFile, gaps); err != nil {
		return fmt.Errorf("write gaps.txt: %w", err)
	}
	log.Infow("wrote gaps", "file", gapsFile, "count", len(gaps))

	if len(gaps) == 0 {
		log.Infow("no gaps to reconcile")
		return nil
	}

	missingFile := filepath.Join(cfg.workdir(), "missing.txt")
	missingF, err := os.OpenFile(missingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open missing.txt: %w", err)
	}
	defer missingF.Close()
	missingW := bufio.NewWriter(missingF)
	defer missingW.Flush()

	const logEvery = 10_000
	var published, recorded int
	reconcileStart := time.Now()
	for i, h := range gaps {
		if err := ctx.Err(); err != nil {
			return err
		}
		e, err := entryForHeight(ctx, hstore, cfg.DataDir, h)
		if err != nil {
			return fmt.Errorf("entry for height %d: %w", h, err)
		}
		ok, err := validHashFile(e.localHash)
		if err != nil {
			return fmt.Errorf("validate hash file for %d: %w", h, err)
		}
		if ok || e.empty {
			if err := publish(e); err != nil {
				return fmt.Errorf("publish %d: %w", h, err)
			}
			published++
			log.Debugw("published existing block", "height", h, "hash_path", e.hashPath, "empty", e.empty)
		} else {
			if _, err := fmt.Fprintf(missingW, "%d\t%s\n", e.height, e.hashPath); err != nil {
				return fmt.Errorf("write missing: %w", err)
			}
			recorded++
			log.Debugw("recorded missing", "height", h, "hash_path", e.hashPath)
		}
		if (i+1)%logEvery == 0 {
			log.Infow("reconcile progress",
				"processed", i+1,
				"total", len(gaps),
				"published", published,
				"missing", recorded,
				"elapsed", time.Since(reconcileStart).Round(time.Second),
			)
		}
	}

	if err := missingW.Flush(); err != nil {
		return fmt.Errorf("flush missing.txt: %w", err)
	}

	log.Infow("reconcile done",
		"published", published,
		"missing", recorded,
		"missing_file", missingFile,
		"elapsed", time.Since(reconcileStart).Round(time.Second),
	)
	return nil
}

// scanHeightsDir returns the heights represented by `<height>.ods` entries in
// the given directory. Non-matching entries are ignored.
func scanHeightsDir(dir string) ([]uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	heights := make([]uint64, 0, len(entries))
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".ods") {
			continue
		}
		num := strings.TrimSuffix(name, ".ods")
		h, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			continue
		}
		heights = append(heights, h)
	}
	return heights, nil
}

// computeGaps returns the sorted set [from..to] minus the heights present in
// `present` (which must be sorted ascending).
func computeGaps(present []uint64, from, to uint64) []uint64 {
	gaps := make([]uint64, 0)
	i := sort.Search(len(present), func(i int) bool { return present[i] >= from })
	for h := from; h <= to; h++ {
		for i < len(present) && present[i] < h {
			i++
		}
		if i < len(present) && present[i] == h {
			continue
		}
		gaps = append(gaps, h)
	}
	return gaps
}

// writeSortedHeights writes the heights (one per line) to a file, truncating it.
func writeSortedHeights(path string, heights []uint64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for _, h := range heights {
		if _, err := fmt.Fprintf(bw, "%d\n", h); err != nil {
			return err
		}
	}
	return bw.Flush()
}
