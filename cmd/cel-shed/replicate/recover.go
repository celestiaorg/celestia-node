package replicate

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"

	libheadstore "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
)

func recoverMissing(ctx context.Context, cfg Config) error {
	hstore, closeStore, err := openHeaderStore(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer closeStore()

	from := cfg.FromHeight
	if from == 0 {
		from = 1
	}
	missing, needRsync, err := scanMissing(ctx, hstore, cfg.DataDir, from, cfg.RecoverMax)
	if err != nil {
		return err
	}

	recoverDir := filepath.Join(cfg.workdir(), "recover")
	if err := os.MkdirAll(recoverDir, 0o755); err != nil {
		return fmt.Errorf("ensure recover dir: %w", err)
	}
	missingPath := filepath.Join(recoverDir, fmt.Sprintf("missing-heights-%d-%d.txt", from, cfg.RecoverMax))
	if err := writeMissingHeights(missingPath, missing); err != nil {
		return err
	}
	log.Infow("recover scan complete",
		"from", from,
		"to", cfg.RecoverMax,
		"missing_heights", len(missing),
		"missing_heights_file", missingPath,
	)
	if len(missing) == 0 {
		return nil
	}

	filesFromPath := filepath.Join(recoverDir, fmt.Sprintf("files-from-%d-%d.txt", from, cfg.RecoverMax))
	if err := writeFilesFromPath(filesFromPath, needRsync); err != nil {
		return err
	}
	log.Infow("recover files-from written",
		"files", len(needRsync),
		"files_from", filesFromPath,
	)
	if len(needRsync) > 0 {
		if err := runRsync(ctx, cfg, filesFromPath); err != nil {
			return err
		}
	}
	if err := publishBatch(ctx, missing); err != nil {
		return err
	}
	if err := verifyEntries(ctx, missing); err != nil {
		return err
	}
	log.Infow("recover done", "recovered_heights", len(missing))
	return nil
}

func scanMissing(
	ctx context.Context,
	hstore *libheadstore.Store[*header.ExtendedHeader],
	dataDir string,
	from, to uint64,
) ([]entry, []entry, error) {
	var missing []entry
	var needRsync []entry
	for height := from; height <= to; height++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		e, err := entryForHeight(ctx, hstore, dataDir, height)
		if err != nil {
			return nil, nil, err
		}
		hashOK, err := validHashFile(e.localHash)
		if err != nil {
			return nil, nil, fmt.Errorf("height %d: %w", height, err)
		}
		linkOK := false
		if hashOK {
			linkOK, err = linkCorrect(e)
			if err != nil {
				return nil, nil, fmt.Errorf("height %d: %w", height, err)
			}
		}
		if hashOK && linkOK {
			continue
		}
		missing = append(missing, e)
		if !hashOK {
			needRsync = append(needRsync, e)
		}
	}
	return missing, needRsync, nil
}

func writeMissingHeights(file string, entries []entry) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("create missing heights file: %w", err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	for _, e := range entries {
		if _, err := fmt.Fprintf(bw, "%d\n", e.height); err != nil {
			return fmt.Errorf("write missing heights: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush missing heights: %w", err)
	}
	return nil
}

func writeFilesFromPath(file string, entries []entry) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("create files-from: %w", err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if _, ok := seen[e.hashPath]; ok {
			continue
		}
		seen[e.hashPath] = struct{}{}
		if _, err := bw.WriteString(e.hashPath + "\n"); err != nil {
			return fmt.Errorf("write files-from: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush files-from: %w", err)
	}
	return nil
}

func verifyEntries(ctx context.Context, entries []entry) error {
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, err := complete(e)
		if err != nil {
			return fmt.Errorf("height %d: verify: %w", e.height, err)
		}
		if !ok {
			return fmt.Errorf("height %d: verify failed", e.height)
		}
	}
	return nil
}
