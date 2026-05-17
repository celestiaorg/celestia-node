package replicate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	logging "github.com/ipfs/go-log/v2"

	libhead "github.com/celestiaorg/go-header"
	libheadstore "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("cmd-shed/replicate")

type entry struct {
	height     uint64
	hashPath   string
	localHash  string
	localLink  string
	linkTarget string
	empty      bool
}

func Run(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
		_ = logging.SetLogLevel("cmd-shed/replicate/headers", cfg.LogLevel)
	}

	headerProg := headers.NewProgress()
	if err := headers.Run(ctx, headers.Config{
		Source:         cfg.Source,
		DataDir:        cfg.DataDir,
		Network:        cfg.Network,
		FromHeight:     cfg.FromHeight,
		ToHeight:       cfg.ToHeight,
		Concurrency:    cfg.Concurrency,
		RequestTimeout: cfg.RequestTimeout,
	}, headerProg); err != nil {
		return fmt.Errorf("headers: %w", err)
	}

	hstore, closeStore, err := openHeaderStore(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer closeStore()

	head, err := hstore.Head(ctx)
	if err != nil {
		if errors.Is(err, libhead.ErrEmptyStore) {
			return fmt.Errorf("header store is empty")
		}
		return fmt.Errorf("read header head: %w", err)
	}
	target := cfg.ToHeight
	if target == 0 {
		target = head.Height()
	}
	if target > head.Height() {
		return fmt.Errorf("target height %d is above local header head %d", target, head.Height())
	}

	scanFrom := cfg.FromHeight
	if scanFrom == 0 {
		scanFrom = 1
	}
	from, ok, err := firstIncomplete(ctx, hstore, cfg.DataDir, scanFrom, target)
	if err != nil {
		return err
	}
	if !ok {
		log.Infow("blocks already complete", "through_height", target)
		if cfg.Verify {
			return verifyComplete(ctx, hstore, cfg.DataDir, scanFrom, target)
		}
		return nil
	}

	startedAt := time.Now()
	for start := from; start <= target; start += cfg.BatchSize {
		end := start + cfg.BatchSize - 1
		if end > target {
			end = target
		}
		entries, err := entriesForRange(ctx, hstore, cfg.DataDir, start, end)
		if err != nil {
			return err
		}
		filesFrom, err := writeFilesFrom(cfg, entries)
		if err != nil {
			return err
		}
		log.Infow("rsync batch", "from", start, "to", end, "files_from", filesFrom)
		if rsyncErr := runRsync(ctx, cfg, filesFrom); rsyncErr != nil {
			missing := missingEntries(entries)
			missingFile, err := appendMissing(cfg, missing)
			if err != nil {
				return fmt.Errorf("record missing entries: %w", err)
			}
			log.Warnw("rsync failed; recording missing entries and continuing",
				"from", start, "to", end, "missing", len(missing), "file", missingFile, "err", rsyncErr)
			continue
		}
		if err := publishBatch(ctx, entries); err != nil {
			return err
		}
		log.Infow("batch done", "through_height", end)
	}
	if cfg.Verify {
		if err := verifyComplete(ctx, hstore, cfg.DataDir, scanFrom, target); err != nil {
			return err
		}
	}
	log.Infow("done", "from", from, "to", target, "headers_stored", headerProg.Stored(),
		"elapsed", time.Since(startedAt).Round(time.Second))
	return nil
}

func openHeaderStore(ctx context.Context, dataDir string) (
	*libheadstore.Store[*header.ExtendedHeader],
	func(),
	error,
) {
	if !nodebuilder.IsInit(dataDir) {
		return nil, nil, fmt.Errorf("data dir %q is not initialised; run `celestia bridge init` first", dataDir)
	}
	nodeStore, err := nodebuilder.OpenStore(dataDir, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open node store: %w", err)
	}
	ds, err := nodeStore.Datastore()
	if err != nil {
		nodeStore.Close()
		return nil, nil, fmt.Errorf("open datastore: %w", err)
	}
	hstore, err := libheadstore.NewStore[*header.ExtendedHeader](ds)
	if err != nil {
		nodeStore.Close()
		return nil, nil, fmt.Errorf("new header store: %w", err)
	}
	if err := hstore.Start(ctx); err != nil {
		nodeStore.Close()
		return nil, nil, fmt.Errorf("start header store: %w", err)
	}
	closeFn := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := hstore.Stop(stopCtx); err != nil {
			log.Warnw("stop header store", "err", err)
		}
		if err := nodeStore.Close(); err != nil {
			log.Warnw("close node store", "err", err)
		}
	}
	return hstore, closeFn, nil
}

func firstIncomplete(
	ctx context.Context,
	hstore *libheadstore.Store[*header.ExtendedHeader],
	dataDir string,
	from, to uint64,
) (uint64, bool, error) {
	for height := from; height <= to; height++ {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		e, err := entryForHeight(ctx, hstore, dataDir, height)
		if err != nil {
			return 0, false, err
		}
		ok, err := complete(e)
		if err != nil {
			return 0, false, err
		}
		if !ok {
			return height, true, nil
		}
	}
	return 0, false, nil
}

func entriesForRange(
	ctx context.Context,
	hstore *libheadstore.Store[*header.ExtendedHeader],
	dataDir string,
	from, to uint64,
) ([]entry, error) {
	entries := make([]entry, 0, to-from+1)
	for height := from; height <= to; height++ {
		e, err := entryForHeight(ctx, hstore, dataDir, height)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func entryForHeight(
	ctx context.Context,
	hstore *libheadstore.Store[*header.ExtendedHeader],
	dataDir string,
	height uint64,
) (entry, error) {
	hdr, err := hstore.GetByHeight(ctx, height)
	if err != nil {
		return entry{}, fmt.Errorf("get header %d: %w", height, err)
	}
	hash := share.DataHash(hdr.DAH.Hash())
	hex := hash.String()
	return entry{
		height:     height,
		hashPath:   "blocks/" + hex + ".ods",
		localHash:  filepath.Join(dataDir, "blocks", hex+".ods"),
		localLink:  filepath.Join(dataDir, "blocks", "heights", fmt.Sprintf("%d.ods", height)),
		linkTarget: "../" + hex + ".ods",
		empty:      hash.IsEmptyEDS(),
	}, nil
}

func writeFilesFrom(cfg Config, entries []entry) (string, error) {
	if err := os.MkdirAll(cfg.workdir(), 0o755); err != nil {
		return "", fmt.Errorf("ensure workdir: %w", err)
	}
	file := filepath.Join(cfg.workdir(), fmt.Sprintf("%d-%d.txt", entries[0].height, entries[len(entries)-1].height))
	f, err := os.Create(file)
	if err != nil {
		return "", fmt.Errorf("create files-from: %w", err)
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
			return "", fmt.Errorf("write files-from: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return "", fmt.Errorf("flush files-from: %w", err)
	}
	return file, nil
}

func runRsync(ctx context.Context, cfg Config, filesFrom string) error {
	remoteSrc := fmt.Sprintf("%s:%s/", cfg.RemoteHost, path.Dir(cfg.RemoteBlocks))
	args := []string{
		"-ah",
		"--partial", "--partial-dir=.rsync-partial",
		"--info=progress2,flist2",
		"--files-from=" + filesFrom,
		remoteSrc,
		cfg.DataDir + string(os.PathSeparator),
	}
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync: %w", err)
	}
	return nil
}

// missingEntries returns the subset of entries whose local hash file is
// absent after a rsync attempt.
func missingEntries(entries []entry) []entry {
	missing := make([]entry, 0)
	for _, e := range entries {
		ok, err := validHashFile(e.localHash)
		if err == nil && !ok {
			missing = append(missing, e)
		}
	}
	return missing
}

// appendMissing appends one line per missing entry ("<height>\t<hashPath>") to
// <workdir>/missing.txt and returns the file path.
func appendMissing(cfg Config, missing []entry) (string, error) {
	if err := os.MkdirAll(cfg.workdir(), 0o755); err != nil {
		return "", fmt.Errorf("ensure workdir: %w", err)
	}
	file := filepath.Join(cfg.workdir(), "missing.txt")
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf("open missing log: %w", err)
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for _, e := range missing {
		if _, err := fmt.Fprintf(bw, "%d\t%s\n", e.height, e.hashPath); err != nil {
			return "", fmt.Errorf("write missing log: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return "", fmt.Errorf("flush missing log: %w", err)
	}
	return file, nil
}

func publishBatch(ctx context.Context, entries []entry) error {
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ok, err := validHashFile(e.localHash); err != nil {
			return fmt.Errorf("height %d: %w", e.height, err)
		} else if !ok {
			return fmt.Errorf("height %d: missing hash file %s", e.height, e.localHash)
		}
		if err := publish(e); err != nil {
			return fmt.Errorf("height %d: %w", e.height, err)
		}
	}
	return nil
}

func verifyComplete(
	ctx context.Context,
	hstore *libheadstore.Store[*header.ExtendedHeader],
	dataDir string,
	from, to uint64,
) error {
	height, ok, err := firstIncomplete(ctx, hstore, dataDir, from, to)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if ok {
		return fmt.Errorf("verify failed: height %d is incomplete", height)
	}
	log.Infow("verify ok", "from", from, "to", to)
	return nil
}

func complete(e entry) (bool, error) {
	ok, err := validHashFile(e.localHash)
	if err != nil || !ok {
		return ok, err
	}
	return linkCorrect(e)
}

func validHashFile(file string) (bool, error) {
	info, err := os.Lstat(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s is a symlink, expected regular file", file)
	}
	return info.Mode().IsRegular() && info.Size() > 0, nil
}

func publish(e entry) error {
	if err := os.MkdirAll(filepath.Dir(e.localLink), 0o755); err != nil {
		return err
	}
	ok, err := linkCorrect(e)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if _, err := os.Lstat(e.localLink); err == nil {
		return fmt.Errorf("%s exists but points at the wrong file", e.localLink)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if e.empty {
		return os.Symlink(e.linkTarget, e.localLink)
	}
	return os.Link(e.localHash, e.localLink)
}

func linkCorrect(e entry) (bool, error) {
	info, err := os.Lstat(e.localLink)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if e.empty {
		if info.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		got, err := os.Readlink(e.localLink)
		return got == e.linkTarget, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}
	hashInfo, err := os.Stat(e.localHash)
	if err != nil {
		return false, err
	}
	return os.SameFile(info, hashInfo), nil
}
