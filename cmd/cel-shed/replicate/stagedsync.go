package replicate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger4"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	libhead_p2p "github.com/celestiaorg/go-header/p2p"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	"github.com/celestiaorg/celestia-node/header"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/store/file"
)

// StagedSyncConfig configures the staged-sync run.
//
// Staged-sync fills the internal gaps of a destination data directory's
// heights/ folder in two phases against an isolated temp/staging directory:
//
//	Phase A (download; skipped when SkipDownload is set):
//	  1. scan <data-dir>/blocks/heights for missing heights (internal gaps).
//	  2. fetch each missing height's header, store it in a badger DB under
//	     <temp-dir>/headers, and append <height>\t<hash> to <temp-dir>/manifest.txt.
//	  3. fetch each missing (non-empty) height's ODS via shrex into
//	     <temp-dir>/blocks/<HASH>.ods.
//
//	Phase B (install; always runs):
//	  4. for every staged height in the manifest, copy
//	     <temp-dir>/blocks/<HASH>.ods -> <data-dir>/blocks/<HASH>.ods and create
//	     the symlink <data-dir>/blocks/heights/<height>.ods -> ../<HASH>.ods.
//
// The temp directory is never deleted; re-running with --skip-download replays
// only Phase B from whatever is already staged.
type StagedSyncConfig struct {
	DataDir          string
	TempDir          string
	Network          modp2p.Network
	FromHeight       uint64
	ToHeight         uint64
	Peers            []string
	Concurrency      int
	RequestTimeout   time.Duration
	MinPeers         int
	DiscoveryTimeout time.Duration
	DiscoveryLimit   uint
	SkipDownload     bool
	Repair           bool
	LogLevel         string
}

func (c StagedSyncConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if strings.TrimSpace(c.TempDir) == "" {
		return fmt.Errorf("temp-dir is required")
	}
	if abs, err := filepath.Abs(c.TempDir); err == nil {
		if dabs, err := filepath.Abs(c.DataDir); err == nil && abs == dabs {
			return fmt.Errorf("temp-dir must differ from data-dir")
		}
	}
	if c.Concurrency < 1 || c.Concurrency > 32 {
		return fmt.Errorf("concurrency must be between 1 and 32, got %d", c.Concurrency)
	}
	if c.RequestTimeout < time.Second {
		return fmt.Errorf("request-timeout must be >= 1s, got %s", c.RequestTimeout)
	}
	if c.MinPeers < 1 {
		return fmt.Errorf("min-peers must be >= 1, got %d", c.MinPeers)
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	for _, p := range c.Peers {
		if _, err := peer.AddrInfoFromString(p); err != nil {
			return fmt.Errorf("invalid --peers entry %q: %w", p, err)
		}
	}
	return nil
}

func (c StagedSyncConfig) headerDBDir() string     { return filepath.Join(c.TempDir, "headers") }
func (c StagedSyncConfig) stagedBlocksDir() string { return filepath.Join(c.TempDir, "blocks") }
func (c StagedSyncConfig) manifestPath() string    { return filepath.Join(c.TempDir, "manifest.txt") }
func (c StagedSyncConfig) remainingPath() string {
	return filepath.Join(c.TempDir, "missing.remaining.txt")
}

// stagedHeight is one entry in the staging manifest: a missing height, its DAH
// hash, and whether the EDS is the canonical empty square (no ODS to fetch).
// roots is the header's DataAvailabilityHeader, needed to decode the shrex ODS
// shares and write them in the store's ODSQ4 file format. It is only populated
// during the download phase (nil on the --skip-download install path).
type stagedHeight struct {
	height uint64
	hash   share.DataHash
	empty  bool
	roots  *share.AxisRoots
}

// RunStagedSync executes Phase A (unless skipped) then Phase B. See
// StagedSyncConfig for the full description.
func RunStagedSync(ctx context.Context, cfg StagedSyncConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}
	if err := os.MkdirAll(cfg.TempDir, 0o755); err != nil {
		return fmt.Errorf("ensure temp-dir: %w", err)
	}

	log.Infow("staged-sync starting",
		"data_dir", cfg.DataDir, "temp_dir", cfg.TempDir, "network", cfg.Network,
		"from_height", cfg.FromHeight, "to_height", cfg.ToHeight,
		"skip_download", cfg.SkipDownload, "repair", cfg.Repair, "concurrency", cfg.Concurrency)
	runStart := time.Now()

	if !cfg.SkipDownload {
		if err := stagedDownload(ctx, cfg); err != nil {
			return err
		}
	} else {
		log.Infow("skip-download set; skipping phase A, using already-staged manifest",
			"manifest", cfg.manifestPath())
	}

	if err := stagedInstall(ctx, cfg); err != nil {
		return err
	}
	log.Infow("staged-sync done", "elapsed", time.Since(runStart).Round(time.Second))
	return nil
}

// stagedDownload runs Phase A: gap scan -> headers into DB + manifest -> ODS
// into the staging blocks dir.
func stagedDownload(ctx context.Context, cfg StagedSyncConfig) error {
	heightsDir := filepath.Join(cfg.DataDir, "blocks", "heights")
	log.Infow("phase A: scanning destination heights dir", "dir", heightsDir)
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
		"present", len(present), "present_min", presentMin, "present_max", presentMax,
		"elapsed", time.Since(scanStart).Round(time.Millisecond))

	// Resolve the scan window. --from-height restricts the search so only
	// heights >= it are considered missing; --to-height caps the top. Each
	// bound is logged with its source so the effective window is unambiguous.
	from, to := cfg.FromHeight, cfg.ToHeight
	fromSrc, toSrc := "--from-height", "--to-height"
	if from == 0 || to == 0 {
		if len(present) == 0 {
			return fmt.Errorf("heights dir is empty and no --from-height/--to-height given; nothing to scan")
		}
		if from == 0 {
			from, fromSrc = presentMin, "present-min (no --from-height)"
		}
		if to == 0 {
			to, toSrc = presentMax, "present-max (no --to-height)"
		}
	}
	if from > to {
		return fmt.Errorf("resolved from-height (%d) > to-height (%d)", from, to)
	}
	log.Infow("resolved gap-scan window",
		"from", from, "from_source", fromSrc,
		"to", to, "to_source", toSrc,
		"heights_in_window", to-from+1)

	var gaps []uint64
	if cfg.Repair {
		// Repair mode: a height needs fetching if it is absent OR present but
		// its block is unreadable by the store (fails OpenODS or lacks a .q4).
		gaps, err = repairTargets(ctx, cfg, from, to)
		if err != nil {
			return fmt.Errorf("repair scan: %w", err)
		}
		log.Infow("repair scan complete",
			"scanned_from", from, "scanned_to", to, "need_fetch", len(gaps))
	} else {
		gaps = computeGaps(present, from, to)
		log.Infow("gap scan complete",
			"scanned_from", from, "scanned_to", to,
			"present_in_window", (to-from+1)-uint64(len(gaps)),
			"missing", len(gaps))
	}
	if len(gaps) == 0 {
		log.Infow("nothing to fetch in window; skipping download phase")
		// Still (re)write an empty manifest so a later --skip-download run is a no-op.
		return writeManifest(cfg.manifestPath(), nil)
	}
	// Log the target heights as compact contiguous runs so the full picture
	// of what will be fetched is visible without one line per height.
	logGapRuns(gaps)

	// libp2p host + peer pool (explicit --peers or archival DHT discovery).
	h, err := headers.NewReplicatorHost()
	if err != nil {
		return fmt.Errorf("new libp2p host: %w", err)
	}
	defer h.Close()
	log.Infow("libp2p host ready", "peer", h.ID().String())

	pool, stopDisc, err := setupPeerPool(ctx, h, cfg)
	if err != nil {
		return err
	}
	if stopDisc != nil {
		defer stopDisc()
	}

	// Phase A.2: fetch headers for the gaps, persist to the badger DB, and
	// build the manifest of staged heights.
	staged, err := fetchAndStoreHeaders(ctx, h, cfg, pool, gaps)
	if err != nil {
		return err
	}
	if err := writeManifest(cfg.manifestPath(), staged); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	log.Infow("headers staged", "count", len(staged), "db", cfg.headerDBDir(), "manifest", cfg.manifestPath())

	// Phase A.3: fetch the ODS for every non-empty staged height into temp.
	return fetchODSToTemp(ctx, h, cfg, pool, staged)
}

// setupPeerPool connects the given --peers or starts archival DHT discovery and
// waits for MinPeers. Returns the pool and an optional discovery-stop func.
func setupPeerPool(ctx context.Context, h host.Host, cfg StagedSyncConfig) (*peerPool, func(), error) {
	pool := newPeerPool()
	if len(cfg.Peers) > 0 {
		for _, p := range cfg.Peers {
			info, _ := peer.AddrInfoFromString(p)
			h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
			h.ConnManager().Protect(info.ID, "staged-sync-peer")
			if err := h.Connect(ctx, *info); err != nil {
				log.Warnw("failed to connect to provided peer", "peer", info.ID, "err", err)
				continue
			}
			pool.add(info.ID)
			log.Infow("connected to provided peer", "peer", info.ID)
		}
		if pool.size() == 0 {
			return nil, nil, fmt.Errorf("none of the provided --peers could be connected")
		}
		return pool, nil, nil
	}

	log.Infow("starting peer discovery", "network", cfg.Network, "topic", archivalTopic)
	stopDisc, err := startArchivalDiscovery(ctx, h, ShrexFetchConfig{
		Network:          cfg.Network,
		DiscoveryLimit:   cfg.DiscoveryLimit,
		DiscoveryTimeout: cfg.DiscoveryTimeout,
	}, pool)
	if err != nil {
		return nil, nil, fmt.Errorf("start discovery: %w", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, cfg.DiscoveryTimeout)
	err = pool.waitForN(waitCtx, cfg.MinPeers)
	cancel()
	if err != nil {
		stopDisc()
		return nil, nil, fmt.Errorf("waiting for %d archival peers: %w", cfg.MinPeers, err)
	}
	log.Infow("discovery ready", "peers", pool.size())
	return pool, stopDisc, nil
}

// fetchAndStoreHeaders fetches a header per gap height via the libhead p2p
// exchange, writes each to the badger DB under <temp>/headers keyed by height,
// and returns the staged heights sorted ascending.
func fetchAndStoreHeaders(
	ctx context.Context,
	h host.Host,
	cfg StagedSyncConfig,
	pool *peerPool,
	gaps []uint64,
) ([]stagedHeight, error) {
	db, err := dsbadger.NewDatastore(cfg.headerDBDir(), nil)
	if err != nil {
		return nil, fmt.Errorf("open header db: %w", err)
	}
	defer db.Close()

	exchange, err := libhead_p2p.NewExchange[*header.ExtendedHeader](
		h, pool.snapshot(), nil,
		libhead_p2p.WithNetworkID[libhead_p2p.ClientParameters](cfg.Network.String()),
		libhead_p2p.WithChainID(cfg.Network.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("new header exchange: %w", err)
	}
	if err := exchange.Start(ctx); err != nil {
		return nil, fmt.Errorf("start header exchange: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exchange.Stop(stopCtx)
	}()

	log.Infow("phase A: fetching headers for missing heights",
		"missing", len(gaps), "peers", pool.size(), "db", cfg.headerDBDir())
	staged := make([]stagedHeight, 0, len(gaps))
	var emptyCount int
	startedAt := time.Now()
	for i, height := range gaps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		hdr, err := exchange.GetByHeight(reqCtx, height)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("get header %d: %w", height, err)
		}
		bin, err := hdr.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshal header %d: %w", height, err)
		}
		if err := db.Put(ctx, headerDBKey(height), bin); err != nil {
			return nil, fmt.Errorf("store header %d: %w", height, err)
		}
		hash := share.DataHash(hdr.DAH.Hash())
		empty := hash.IsEmptyEDS()
		if empty {
			emptyCount++
		}
		staged = append(staged, stagedHeight{height: height, hash: hash, empty: empty, roots: hdr.DAH})
		log.Infow("header fetched",
			"progress", fmt.Sprintf("%d/%d", i+1, len(gaps)),
			"height", height, "hash", hash.String(), "empty", empty)
		if (i+1)%1000 == 0 {
			log.Infow("headers progress",
				"fetched", i+1, "total", len(gaps), "empty_so_far", emptyCount,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}
	if err := db.Sync(ctx, datastore.NewKey("/hdr")); err != nil {
		return nil, fmt.Errorf("sync header db: %w", err)
	}
	log.Infow("all headers fetched and stored",
		"count", len(staged), "empty_squares", emptyCount,
		"non_empty", len(staged)-emptyCount,
		"elapsed", time.Since(startedAt).Round(time.Second))
	return staged, nil
}

// repairTargets scans [from..to] and returns every height that needs
// (re)fetching: absent heights plus present heights whose block is not
// store-readable (fails file.OpenODS or has no sibling .q4). This is what lets
// --repair fix blocks written by the old raw-ODS path, which still have a
// present height link and so are invisible to a plain gap scan.
func repairTargets(ctx context.Context, cfg StagedSyncConfig, from, to uint64) ([]uint64, error) {
	heightsDir := filepath.Join(cfg.DataDir, "blocks", "heights")
	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	emptyHex := hex.EncodeToString(share.EmptyEDSDataHash())

	var targets []uint64
	var checked, absent, broken, valid uint64
	startedAt := time.Now()
	log.Infow("repair: validating present blocks", "from", from, "to", to)
	for h := from; h <= to; h++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		checked++
		linkPath := filepath.Join(heightsDir, strconv.FormatUint(h, 10)+".ods")
		li, err := os.Lstat(linkPath)
		switch {
		case err == nil:
			ok, reason := validateBlock(ctx, blocksDir, linkPath, li, emptyHex)
			if ok {
				valid++
			} else {
				broken++
				targets = append(targets, h)
				log.Infow("repair: block needs refetch", "height", h, "reason", reason)
			}
		case errors.Is(err, os.ErrNotExist):
			absent++
			targets = append(targets, h)
			log.Debugw("repair: height absent", "height", h)
		default:
			return nil, fmt.Errorf("lstat height %d: %w", h, err)
		}
		if checked%5000 == 0 {
			log.Infow("repair scan progress",
				"checked", checked, "of", to-from+1,
				"absent", absent, "broken", broken, "valid", valid,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}
	log.Infow("repair scan summary",
		"checked", checked, "absent", absent, "broken", broken, "valid", valid,
		"need_fetch", len(targets), "elapsed", time.Since(startedAt).Round(time.Second))
	return targets, nil
}

// validateBlock reports whether the block behind a present height link is
// store-readable. Empty-EDS heights (symlinks to the canonical empty ODS the
// node manages) are treated as valid.
func validateBlock(
	ctx context.Context,
	blocksDir, linkPath string,
	li os.FileInfo,
	emptyHex string,
) (bool, string) {
	if li.Mode()&os.ModeSymlink != 0 {
		if tgt, err := os.Readlink(linkPath); err == nil {
			base := strings.TrimSuffix(filepath.Base(tgt), ".ods")
			if strings.EqualFold(base, emptyHex) {
				return true, ""
			}
		}
	}
	ods, err := file.OpenODS(linkPath)
	if err != nil {
		return false, fmt.Sprintf("OpenODS failed: %v", err)
	}
	dh, err := ods.DataHash(ctx)
	_ = ods.Close()
	if err != nil {
		return false, fmt.Sprintf("DataHash failed: %v", err)
	}
	q4 := filepath.Join(blocksDir, dh.String()+".q4")
	if info, err := os.Stat(q4); err != nil || info.Size() == 0 {
		return false, "missing .q4"
	}
	return true, ""
}

// logGapRuns logs the missing heights collapsed into contiguous [start-end]
// runs so the full set is visible compactly. gaps must be sorted ascending.
func logGapRuns(gaps []uint64) {
	if len(gaps) == 0 {
		return
	}
	type run struct{ start, end uint64 }
	runs := make([]run, 0)
	start, prev := gaps[0], gaps[0]
	for _, h := range gaps[1:] {
		if h == prev+1 {
			prev = h
			continue
		}
		runs = append(runs, run{start, prev})
		start, prev = h, h
	}
	runs = append(runs, run{start, prev})

	log.Infow("missing heights collapsed into runs", "runs", len(runs), "total_missing", len(gaps))
	const maxRuns = 200
	for i, r := range runs {
		if i >= maxRuns {
			log.Infow("missing-run listing truncated",
				"shown", maxRuns, "remaining_runs", len(runs)-maxRuns)
			break
		}
		if r.start == r.end {
			log.Infow("missing run", "n", i+1, "height", r.start, "count", 1)
		} else {
			log.Infow("missing run", "n", i+1,
				"from", r.start, "to", r.end, "count", r.end-r.start+1)
		}
	}
}

// fetchODSToTemp downloads the ODS for every non-empty staged height into
// <temp>/blocks/<HASH>.ods concurrently. Empty squares are skipped (Phase B
// materialises them as empty-EDS symlinks). Heights that fail are recorded to
// <temp>/missing.remaining.txt.
func fetchODSToTemp(
	ctx context.Context,
	h host.Host,
	cfg StagedSyncConfig,
	pool *peerPool,
	staged []stagedHeight,
) error {
	if err := os.MkdirAll(cfg.stagedBlocksDir(), 0o755); err != nil {
		return fmt.Errorf("ensure staged blocks dir: %w", err)
	}
	clientParams := shrex.DefaultClientParameters()
	clientParams.WithNetworkID(cfg.Network.String())
	client, err := shrex.NewClient(clientParams, h)
	if err != nil {
		return fmt.Errorf("new shrex client: %w", err)
	}

	remainingF, err := os.OpenFile(cfg.remainingPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open missing.remaining.txt: %w", err)
	}
	defer remainingF.Close()
	remainingW := bufio.NewWriter(remainingF)
	remainingMu := &sync.Mutex{}

	var (
		fetched, skipped, failed int
		totalBytes               int64
		counter                  sync.Mutex
		wg                       sync.WaitGroup
	)
	nonEmpty := 0
	for _, sh := range staged {
		if !sh.empty {
			nonEmpty++
		}
	}
	log.Infow("phase A.3: fetching ODS via shrex into staging",
		"to_fetch", nonEmpty, "empty_to_skip", len(staged)-nonEmpty,
		"concurrency", cfg.Concurrency, "dir", cfg.stagedBlocksDir())
	sem := make(chan struct{}, cfg.Concurrency)
	startedAt := time.Now()
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-progressCtx.Done():
				return
			case <-t.C:
				counter.Lock()
				log.Infow("staged ODS fetch progress",
					"fetched", fetched, "skipped_empty", skipped, "failed", failed,
					"total", len(staged), "peers", pool.size(),
					"bytes_total", totalBytes,
					"avg_mb_per_s", mbPerSec(totalBytes, time.Since(startedAt)),
					"elapsed", time.Since(startedAt).Round(time.Second))
				counter.Unlock()
			}
		}
	}()

	for _, sh := range staged {
		if err := ctx.Err(); err != nil {
			break
		}
		if sh.empty {
			counter.Lock()
			skipped++
			counter.Unlock()
			log.Infow("empty square; no ODS to fetch",
				"height", sh.height, "hash", sh.hash.String())
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(sh stagedHeight) {
			defer wg.Done()
			defer func() { <-sem }()

			res := fetchOneODS(ctx, client, pool, cfg, sh)
			counter.Lock()
			if !res.ok {
				failed++
				counter.Unlock()
				log.Warnw("staged ODS fetch failed after all peers",
					"height", sh.height, "hash", sh.hash.String(),
					"attempts", res.attempts, "last_err", res.lastErr)
				remainingMu.Lock()
				_, _ = fmt.Fprintf(remainingW, "%d\t%s\n", sh.height, sh.hash.String())
				remainingMu.Unlock()
				return
			}
			fetched++
			totalBytes += int64(res.bytes)
			doneNow, bytesNow := fetched, totalBytes
			counter.Unlock()
			if !res.alreadyOnDisk {
				log.Infow("staged ODS fetched",
					"height", sh.height, "hash", sh.hash.String(),
					"bytes", res.bytes, "attempts", res.attempts, "peer", res.peer.String(),
					"fetched_total", doneNow, "of_non_empty", nonEmpty,
					"bytes_total", bytesNow)
			}
		}(sh)
	}
	wg.Wait()
	if err := remainingW.Flush(); err != nil {
		log.Warnw("flush missing.remaining.txt failed", "err", err)
	}

	log.Infow("staged ODS fetch done",
		"fetched", fetched, "skipped_empty", skipped, "failed", failed,
		"total", len(staged), "bytes_total", totalBytes,
		"avg_mb_per_s", mbPerSec(totalBytes, time.Since(startedAt)),
		"elapsed", time.Since(startedAt).Round(time.Second))
	if failed > 0 {
		return fmt.Errorf("staged download finished with %d failures (see %s)", failed, cfg.remainingPath())
	}
	return nil
}

// odsResult carries the outcome of a single ODS fetch for logging.
type odsResult struct {
	ok            bool
	bytes         int
	attempts      int
	peer          peer.ID
	alreadyOnDisk bool
	lastErr       error
}

// fetchOneODS fetches a single height's ODS via shrex, retrying across peers,
// then writes it in the store's ODSQ4 format (both <hash>.ods and <hash>.q4)
// under the staging blocks dir. shrex returns raw ODS shares in row-major order;
// they are decoded with eds.ReadAccessor (which also verifies the datahash
// against the header roots) and re-encoded via file.CreateODSQ4 so the files are
// readable by a node's store. A valid pair already on disk short-circuits.
func fetchOneODS(
	ctx context.Context,
	client *shrex.Client,
	pool *peerPool,
	cfg StagedSyncConfig,
	sh stagedHeight,
) odsResult {
	odsPath := filepath.Join(cfg.stagedBlocksDir(), sh.hash.String()+".ods")
	q4Path := filepath.Join(cfg.stagedBlocksDir(), sh.hash.String()+".q4")

	if odsInfo, err := os.Stat(odsPath); err == nil && odsInfo.Size() > 0 {
		if q4Info, err := os.Stat(q4Path); err == nil && q4Info.Size() > 0 {
			log.Infow("staged ODSQ4 already present; skipping fetch",
				"height", sh.height, "hash", sh.hash.String(), "ods_bytes", odsInfo.Size())
			return odsResult{ok: true, bytes: int(odsInfo.Size()), alreadyOnDisk: true}
		}
	}
	if sh.roots == nil {
		return odsResult{lastErr: fmt.Errorf("height %d: missing header roots", sh.height)}
	}

	edsID, err := shwap.NewEdsID(sh.height)
	if err != nil {
		return odsResult{lastErr: fmt.Errorf("new eds id: %w", err)}
	}
	tried := make(map[peer.ID]struct{})
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return odsResult{attempts: attempt - 1, lastErr: err}
		}
		p, ok := pool.pickExcluding(tried)
		if !ok {
			return odsResult{attempts: attempt - 1, lastErr: lastErr}
		}
		tried[p] = struct{}{}

		reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		buff := bytes.NewBuffer(nil)
		getStart := time.Now()
		err := client.Get(reqCtx, &edsID, buff, p)
		cancel()
		if err != nil {
			lastErr = err
			log.Infow("shrex get attempt failed",
				"height", sh.height, "hash", sh.hash.String(), "attempt", attempt,
				"peer", p.String(), "elapsed", time.Since(getStart).Round(time.Millisecond),
				"err", err)
			continue
		}

		rawBytes := buff.Len()
		// Decode the raw ODS shares into a full EDS. This also verifies the
		// content hash matches the header roots, so a corrupt/wrong response
		// is rejected here rather than silently written.
		square, err := eds.ReadAccessor(ctx, bytes.NewReader(buff.Bytes()), sh.roots)
		if err != nil {
			lastErr = fmt.Errorf("decode ods shares: %w", err)
			log.Warnw("decoding staged ODS failed (retrying)",
				"height", sh.height, "hash", sh.hash.String(), "peer", p.String(), "err", err)
			continue
		}
		// Overwrite any stale partials (CreateODS uses O_EXCL).
		_ = os.Remove(odsPath)
		_ = os.Remove(q4Path)
		if err := file.CreateODSQ4(odsPath, q4Path, sh.roots, square.ExtendedDataSquare); err != nil {
			lastErr = fmt.Errorf("write odsq4: %w", err)
			log.Warnw("writing staged ODSQ4 failed",
				"height", sh.height, "ods", odsPath, "err", err)
			_ = os.Remove(odsPath)
			_ = os.Remove(q4Path)
			continue
		}
		odsInfo, _ := os.Stat(odsPath)
		var odsBytes int
		if odsInfo != nil {
			odsBytes = int(odsInfo.Size())
		}
		log.Debugw("wrote staged ODSQ4",
			"height", sh.height, "hash", sh.hash.String(),
			"raw_shares_bytes", rawBytes, "ods_file_bytes", odsBytes)
		return odsResult{ok: true, bytes: odsBytes, attempts: attempt, peer: p}
	}
}

// stagedInstall runs Phase B: for every manifest entry, copy the staged ODS
// into the destination blocks dir and create the height symlink.
func stagedInstall(ctx context.Context, cfg StagedSyncConfig) error {
	staged, err := readManifest(cfg.manifestPath())
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if len(staged) == 0 {
		log.Infow("manifest is empty; nothing to install")
		return nil
	}
	blocksDir := filepath.Join(cfg.DataDir, "blocks")
	heightsDir := filepath.Join(blocksDir, "heights")
	if err := os.MkdirAll(heightsDir, 0o755); err != nil {
		return fmt.Errorf("ensure heights dir: %w", err)
	}

	log.Infow("phase B: installing staged heights into destination",
		"count", len(staged), "blocks_dir", blocksDir, "heights_dir", heightsDir)
	var installed, relinked, missingStaged int
	startedAt := time.Now()
	for i, sh := range staged {
		if err := ctx.Err(); err != nil {
			return err
		}
		hashHex := sh.hash.String()
		destHash := filepath.Join(blocksDir, hashHex+".ods")
		destLink := filepath.Join(heightsDir, fmt.Sprintf("%d.ods", sh.height))

		copied := false
		if !sh.empty {
			srcODS := filepath.Join(cfg.stagedBlocksDir(), hashHex+".ods")
			srcQ4 := filepath.Join(cfg.stagedBlocksDir(), hashHex+".q4")
			destQ4 := filepath.Join(blocksDir, hashHex+".q4")
			// Both ODS and Q4 must be staged; the store needs the pair.
			odsInfo, odsErr := os.Stat(srcODS)
			q4Info, q4Err := os.Stat(srcQ4)
			if odsErr != nil || odsInfo.Size() == 0 || q4Err != nil || q4Info.Size() == 0 {
				missingStaged++
				log.Warnw("staged ODSQ4 incomplete; skipping height",
					"progress", fmt.Sprintf("%d/%d", i+1, len(staged)),
					"height", sh.height, "ods", srcODS, "ods_ok", odsErr == nil,
					"q4", srcQ4, "q4_ok", q4Err == nil)
				continue
			}
			// The destination is only "good" if BOTH files are present. A
			// missing/empty .q4 is the tell of a block written by the old
			// raw-ODS fetch: overwrite the pair so such blocks are repaired.
			dODS, dODSErr := os.Stat(destHash)
			dQ4, dQ4Err := os.Stat(destQ4)
			destComplete := dODSErr == nil && dODS.Size() > 0 && dQ4Err == nil && dQ4.Size() > 0
			if !destComplete {
				_ = os.Remove(destHash)
				_ = os.Remove(destQ4)
				if err := copyFileAtomic(srcODS, destHash); err != nil {
					return fmt.Errorf("copy ODS %d: %w", sh.height, err)
				}
				if err := copyFileAtomic(srcQ4, destQ4); err != nil {
					return fmt.Errorf("copy Q4 %d: %w", sh.height, err)
				}
				installed++
				copied = true
			}
		}

		// Link the height using the store convention: hardlink for non-empty,
		// symlink for empty. Fixes a pre-existing symlink left by an older run.
		created, err := ensureStoreLink(blocksDir, heightsDir, sh.height, sh.hash)
		if err != nil {
			return fmt.Errorf("link %d: %w", sh.height, err)
		}
		if created {
			relinked++
		}
		log.Infow("installed height",
			"progress", fmt.Sprintf("%d/%d", i+1, len(staged)),
			"height", sh.height, "hash", hashHex, "empty", sh.empty,
			"copied_block", copied, "linked", created, "link", destLink)
		if (i+1)%5000 == 0 {
			log.Infow("install progress", "processed", i+1, "total", len(staged),
				"copied", installed, "linked", relinked,
				"elapsed", time.Since(startedAt).Round(time.Second))
		}
	}
	log.Infow("install done",
		"copied", installed, "linked", relinked, "missing_staged", missingStaged,
		"total", len(staged), "elapsed", time.Since(startedAt).Round(time.Second))
	if missingStaged > 0 {
		return fmt.Errorf("install finished with %d heights missing from staging (run without --skip-download to fetch them)", missingStaged)
	}
	return nil
}

func headerDBKey(height uint64) datastore.Key {
	return datastore.NewKey(fmt.Sprintf("/hdr/%d", height))
}

// ensureStoreLink makes heights/<height>.ods match the store's linking
// convention: a hardlink to blocks/<hash>.ods for a non-empty block, or a
// symlink to ../<hash>.ods for the empty EDS (the store hardlinks non-empty and
// symlinks empty). It is idempotent and repairs a link of the wrong kind — in
// particular a symlink left where a hardlink belongs. Returns true when it
// created or replaced the link.
func ensureStoreLink(blocksDir, heightsDir string, height uint64, hash share.DataHash) (bool, error) {
	linkPath := filepath.Join(heightsDir, strconv.FormatUint(height, 10)+".ods")

	if hash.IsEmptyEDS() {
		target := "../" + hash.String() + ".ods"
		if cur, err := os.Readlink(linkPath); err == nil && cur == target {
			return false, nil // already the correct symlink
		}
		if _, err := os.Lstat(linkPath); err == nil {
			if err := os.Remove(linkPath); err != nil {
				return false, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return true, os.Symlink(target, linkPath)
	}

	blockPath := filepath.Join(blocksDir, hash.String()+".ods")
	if li, err := os.Lstat(linkPath); err == nil {
		// A regular file sharing the block's inode is already a correct hardlink.
		if li.Mode()&os.ModeSymlink == 0 {
			if bi, err := os.Stat(blockPath); err == nil && os.SameFile(li, bi) {
				return false, nil
			}
		}
		if err := os.Remove(linkPath); err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, os.Link(blockPath, linkPath)
}

// copyFileAtomic copies src to <dst>.tmp, fsyncs, and renames to dst.
func copyFileAtomic(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	_ = os.Remove(tmp)
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func writeManifest(path string, staged []stagedHeight) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	for _, sh := range staged {
		empty := 0
		if sh.empty {
			empty = 1
		}
		if _, err := fmt.Fprintf(bw, "%d\t%s\t%d\n", sh.height, sh.hash.String(), empty); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func readManifest(path string) ([]stagedHeight, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("manifest %s not found; run without --skip-download first", path)
		}
		return nil, err
	}
	defer f.Close()
	var out []stagedHeight
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		height, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		hashBytes, err := hex.DecodeString(fields[1])
		if err != nil || len(hashBytes) != share.DataHashSize {
			return nil, fmt.Errorf("manifest line %q: bad hash", line)
		}
		empty := false
		if len(fields) >= 3 {
			empty = fields[2] == "1"
		}
		out = append(out, stagedHeight{
			height: height,
			hash:   share.DataHash(hashBytes),
			empty:  empty,
		})
	}
	return out, sc.Err()
}
