package replicate

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

// ShrexFetchConfig configures the shrex-fetch run.
type ShrexFetchConfig struct {
	DataDir          string
	Network          modp2p.Network
	MissingFile      string
	FromHeight       uint64
	ToHeight         uint64
	OutDir           string
	Peers            []string
	Concurrency      int
	RequestTimeout   time.Duration
	MinPeers         int
	DiscoveryTimeout time.Duration
	DiscoveryLimit   uint
	LogLevel         string
}

func (c ShrexFetchConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
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
	rangeSet := c.FromHeight != 0 || c.ToHeight != 0
	if rangeSet {
		if c.FromHeight == 0 || c.ToHeight == 0 {
			return fmt.Errorf("from-height and to-height must both be set, or neither")
		}
		if c.FromHeight > c.ToHeight {
			return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
		}
		if c.MissingFile != "" {
			return fmt.Errorf("--missing-file and --from-height/--to-height are mutually exclusive")
		}
	}
	for _, p := range c.Peers {
		if _, err := peer.AddrInfoFromString(p); err != nil {
			return fmt.Errorf("invalid --peers entry %q: %w", p, err)
		}
	}
	return nil
}

func (c ShrexFetchConfig) workdir() string {
	return filepath.Join(c.DataDir, ".cel-shed-replicate")
}

// RunShrexFetch fetches ODS bytes for a set of heights via shrex.
//
// Input modes (mutually exclusive):
//   - missing.txt mode (default): each line `<height>\t<hashPath>`; bytes saved
//     to `<data-dir>/<hashPath>` (typically `<data-dir>/blocks/<HASH>.ods`).
//   - range mode: --from-height/--to-height enumerate the range; bytes saved to
//     `<out-dir>/<height>.ods` (default out-dir: `<data-dir>/blocks/by-height/`).
//
// Heights that fail to fetch are written to missing.remaining.txt. The header
// store is NOT consulted; wire bytes are written verbatim.
func RunShrexFetch(ctx context.Context, cfg ShrexFetchConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	items, missingPath, err := buildItems(cfg)
	if err != nil {
		return err
	}
	if missingPath != "" {
		log.Infow("loaded missing list", "file", missingPath, "count", len(items))
	} else {
		log.Infow("range mode", "from", cfg.FromHeight, "to", cfg.ToHeight,
			"out_dir", rangeOutDir(cfg), "count", len(items))
	}
	if len(items) == 0 {
		log.Infow("nothing to fetch")
		return nil
	}

	h, err := headers.NewReplicatorHost()
	if err != nil {
		return fmt.Errorf("new libp2p host: %w", err)
	}
	defer h.Close()
	log.Infow("libp2p host ready", "peer", h.ID().String())

	pool := newPeerPool()
	var stopDisc func()
	if len(cfg.Peers) > 0 {
		for _, p := range cfg.Peers {
			info, _ := peer.AddrInfoFromString(p)
			h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
			h.ConnManager().Protect(info.ID, "shrex-fetch-peer")
			if err := h.Connect(ctx, *info); err != nil {
				log.Warnw("failed to connect to provided peer", "peer", info.ID, "err", err)
				continue
			}
			pool.add(info.ID)
			log.Infow("connected to provided peer", "peer", info.ID)
		}
		if pool.size() == 0 {
			return fmt.Errorf("none of the provided --peers could be connected")
		}
	} else {
		log.Infow("starting peer discovery", "network", cfg.Network, "topic", archivalTopic)
		stopDisc, err = startArchivalDiscovery(ctx, h, cfg, pool)
		if err != nil {
			return fmt.Errorf("start discovery: %w", err)
		}
		defer stopDisc()

		waitCtx, cancel := context.WithTimeout(ctx, cfg.DiscoveryTimeout)
		err := pool.waitForN(waitCtx, cfg.MinPeers)
		cancel()
		if err != nil {
			return fmt.Errorf("waiting for %d archival peers: %w", cfg.MinPeers, err)
		}
		log.Infow("discovery ready", "peers", pool.size())
	}

	clientParams := shrex.DefaultClientParameters()
	clientParams.WithNetworkID(cfg.Network.String())
	client, err := shrex.NewClient(clientParams, h)
	if err != nil {
		return fmt.Errorf("new shrex client: %w", err)
	}

	if err := os.MkdirAll(cfg.workdir(), 0o755); err != nil {
		return fmt.Errorf("ensure workdir: %w", err)
	}
	remainingPath := filepath.Join(cfg.workdir(), "missing.remaining.txt")
	remainingF, err := os.OpenFile(remainingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open missing.remaining.txt: %w", err)
	}
	defer remainingF.Close()
	remainingW := bufio.NewWriter(remainingF)
	remainingMu := &sync.Mutex{}

	var (
		fetched    int
		failed     int
		totalBytes int64
		counter    sync.Mutex
		wg         sync.WaitGroup
	)
	sem := make(chan struct{}, cfg.Concurrency)
	progress := time.NewTicker(30 * time.Second)
	defer progress.Stop()
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()
	startedAt := time.Now()
	go func() {
		for {
			select {
			case <-progressCtx.Done():
				return
			case <-progress.C:
				counter.Lock()
				elapsed := time.Since(startedAt)
				log.Infow("shrex fetch progress",
					"fetched", fetched, "failed", failed,
					"total", len(items),
					"peers", pool.size(),
					"bytes_total", totalBytes,
					"avg_mb_per_s", mbPerSec(totalBytes, elapsed),
					"elapsed", elapsed.Round(time.Second),
				)
				counter.Unlock()
			}
		}
	}()

	for _, it := range items {
		if err := ctx.Err(); err != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(it fetchItem) {
			defer wg.Done()
			defer func() { <-sem }()

			if info, err := os.Lstat(it.outPath); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
				log.Infow("already on disk; skipping", "height", it.height, "path", it.outPath)
				counter.Lock()
				fetched++
				counter.Unlock()
				return
			}

			fetchStart := time.Now()
			ok, attempts, bytesGot, lastErr := fetchToDisk(ctx, client, pool, cfg, it)
			if !ok {
				log.Warnw("fetch failed all peers",
					"height", it.height, "attempts", attempts, "last_err", lastErr)
				recordRemaining(remainingW, remainingMu, it.height, it.hashPath)
				counter.Lock()
				failed++
				counter.Unlock()
				return
			}
			fetchElapsed := time.Since(fetchStart)
			counter.Lock()
			fetched++
			totalBytes += int64(bytesGot)
			snapBytes := totalBytes
			snapFetched := fetched
			counter.Unlock()
			runElapsed := time.Since(startedAt)
			log.Infow("fetched and persisted",
				"height", it.height, "path", it.outPath,
				"bytes", bytesGot, "attempts", attempts,
				"elapsed", fetchElapsed.Round(time.Millisecond),
				"mb_per_s", mbPerSec(int64(bytesGot), fetchElapsed),
				"avg_mb_per_s", mbPerSec(snapBytes, runElapsed),
				"fetched_total", snapFetched,
				"bytes_total", snapBytes,
			)
		}(it)
	}
	wg.Wait()
	if err := remainingW.Flush(); err != nil {
		log.Warnw("flush missing.remaining.txt failed", "err", err)
	}

	totalElapsed := time.Since(startedAt)
	log.Infow("shrex fetch done",
		"fetched", fetched, "failed", failed,
		"total", len(items),
		"bytes_total", totalBytes,
		"avg_mb_per_s", mbPerSec(totalBytes, totalElapsed),
		"remaining_file", remainingPath,
		"elapsed", totalElapsed.Round(time.Second),
	)
	if failed > 0 {
		return fmt.Errorf("shrex fetch finished with %d failures (see %s)", failed, remainingPath)
	}
	return nil
}

// mbPerSec returns the throughput in MB/s (decimal megabytes), rounded to 2
// decimal places. Returns 0 for non-positive elapsed.
func mbPerSec(bytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	mb := float64(bytes) / 1_000_000.0
	rate := mb / elapsed.Seconds()
	return float64(int64(rate*100)) / 100
}

// fetchItem represents one unit of work for shrex-fetch: a height, the
// absolute on-disk path to write to, and the optional hashPath (only set when
// derived from missing.txt; used for missing.remaining.txt formatting).
type fetchItem struct {
	height   uint64
	outPath  string
	hashPath string
}

// rangeOutDir returns the directory used for range-mode output: --out-dir if
// set, else `<data-dir>/blocks/by-height/`.
func rangeOutDir(cfg ShrexFetchConfig) string {
	if cfg.OutDir != "" {
		return cfg.OutDir
	}
	return filepath.Join(cfg.DataDir, "blocks", "by-height")
}

// buildItems prepares the list of heights to fetch and their output paths.
// Returns (items, missingPath) — missingPath is "" in range mode.
func buildItems(cfg ShrexFetchConfig) ([]fetchItem, string, error) {
	if cfg.FromHeight != 0 && cfg.ToHeight != 0 {
		outDir := rangeOutDir(cfg)
		items := make([]fetchItem, 0, cfg.ToHeight-cfg.FromHeight+1)
		for h := cfg.FromHeight; h <= cfg.ToHeight; h++ {
			items = append(items, fetchItem{
				height:  h,
				outPath: filepath.Join(outDir, fmt.Sprintf("%d.ods", h)),
			})
		}
		return items, "", nil
	}

	missingPath := cfg.MissingFile
	if missingPath == "" {
		missingPath = filepath.Join(cfg.workdir(), "missing.txt")
	}
	items, err := readMissingItems(missingPath, cfg.DataDir)
	if err != nil {
		return nil, "", fmt.Errorf("read missing list: %w", err)
	}
	return items, missingPath, nil
}

// fetchToDisk requests the EDS wire bytes for the given height from peers in
// the pool (retrying across peers until success or all peers exhausted) and
// writes the raw bytes to `outPath` (`<outPath>.tmp` -> fsync -> rename).
// Returns (success, attempts, bytesWritten, lastErr).
func fetchToDisk(
	ctx context.Context,
	client *shrex.Client,
	pool *peerPool,
	cfg ShrexFetchConfig,
	it fetchItem,
) (bool, int, int, error) {
	edsID, err := shwap.NewEdsID(it.height)
	if err != nil {
		return false, 0, 0, fmt.Errorf("new eds id: %w", err)
	}

	tried := make(map[peer.ID]struct{})
	var lastErr error
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return false, attempt, 0, err
		}
		p, ok := pool.pickExcluding(tried)
		if !ok {
			return false, attempt, 0, lastErr
		}
		tried[p] = struct{}{}

		log.Infow("shrex get start", "height", it.height, "peer", p, "attempt", attempt+1)
		reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		buff := bytes.NewBuffer(nil)
		getStart := time.Now()
		err := client.Get(reqCtx, &edsID, buff, p)
		cancel()
		if err != nil {
			lastErr = err
			log.Infow("shrex get failed",
				"height", it.height, "peer", p,
				"elapsed", time.Since(getStart).Round(time.Millisecond),
				"err", err)
			continue
		}
		log.Infow("shrex get ok",
			"height", it.height, "peer", p,
			"bytes", buff.Len(),
			"elapsed", time.Since(getStart).Round(time.Millisecond))

		if err := writeBytesAtomic(it.outPath, buff.Bytes()); err != nil {
			lastErr = fmt.Errorf("write: %w", err)
			log.Warnw("write failed", "height", it.height, "path", it.outPath, "err", err)
			continue
		}
		return true, attempt + 1, buff.Len(), nil
	}
}

// writeBytesAtomic writes data to <path>.tmp, fsyncs, and renames to <path>.
// Creates parent directories as needed.
func writeBytesAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readMissingItems(path, dataDir string) ([]fetchItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var items []fetchItem
	seen := make(map[uint64]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		h, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		items = append(items, fetchItem{
			height:   h,
			outPath:  filepath.Join(dataDir, fields[1]),
			hashPath: fields[1],
		})
	}
	return items, sc.Err()
}

func recordRemaining(w *bufio.Writer, mu *sync.Mutex, height uint64, hashPath string) {
	mu.Lock()
	defer mu.Unlock()
	if hashPath == "" {
		_, _ = fmt.Fprintf(w, "%d\n", height)
		return
	}
	_, _ = fmt.Fprintf(w, "%d\t%s\n", height, hashPath)
}

// peerPool is a thread-safe set of peer IDs for round-robin selection,
// with notification on additions.
type peerPool struct {
	mu     sync.Mutex
	peers  []peer.ID
	notify []chan struct{}
}

func newPeerPool() *peerPool {
	return &peerPool{}
}

func (p *peerPool) add(id peer.ID) {
	p.mu.Lock()
	for _, existing := range p.peers {
		if existing == id {
			p.mu.Unlock()
			return
		}
	}
	p.peers = append(p.peers, id)
	chans := p.notify
	p.notify = nil
	p.mu.Unlock()
	for _, ch := range chans {
		close(ch)
	}
}

func (p *peerPool) remove(id peer.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, existing := range p.peers {
		if existing == id {
			p.peers = append(p.peers[:i], p.peers[i+1:]...)
			return
		}
	}
}

func (p *peerPool) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.peers)
}

// pickExcluding returns a peer not present in `exclude`. Returns false if all
// known peers are excluded.
func (p *peerPool) pickExcluding(exclude map[peer.ID]struct{}) (peer.ID, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.peers) == 0 {
		return "", false
	}
	start := rand.IntN(len(p.peers))
	for i := 0; i < len(p.peers); i++ {
		pid := p.peers[(start+i)%len(p.peers)]
		if _, skip := exclude[pid]; skip {
			continue
		}
		return pid, true
	}
	return "", false
}

func (p *peerPool) waitForN(ctx context.Context, n int) error {
	for {
		p.mu.Lock()
		if len(p.peers) >= n {
			p.mu.Unlock()
			return nil
		}
		ch := make(chan struct{})
		p.notify = append(p.notify, ch)
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}
