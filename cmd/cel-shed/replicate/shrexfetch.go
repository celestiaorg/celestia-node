package replicate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	libhead_p2p "github.com/celestiaorg/go-header/p2p"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	"github.com/celestiaorg/celestia-node/header"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
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
		if c.FromHeight <= 1 {
			return fmt.Errorf("from-height must be > 1 (anchor needs height-1)")
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

// RunShrexFetch fetches ODS bytes for a set of heights via shrex and writes
// them to <data-dir>/blocks/<HASH>.ods, then creates the height link at
// <data-dir>/blocks/heights/<height>.ods — same layout as `replicate`.
//
// Input modes (mutually exclusive):
//   - missing.txt mode (default): each line `<height>\t<hashPath>`; hash comes
//     from the hashPath column.
//   - range mode: --from-height/--to-height; headers (and hashes) are fetched
//     from the same peer pool via libhead p2p exchange.
//
// Heights that fail to fetch are written to missing.remaining.txt. The local
// header store is NOT opened.
func RunShrexFetch(ctx context.Context, cfg ShrexFetchConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
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

	items, missingPath, err := buildItems(ctx, h, cfg, pool)
	if err != nil {
		return err
	}
	if missingPath != "" {
		log.Infow("loaded missing list", "file", missingPath, "count", len(items))
	} else {
		log.Infow("range prepared", "from", cfg.FromHeight, "to", cfg.ToHeight, "count", len(items))
	}
	if len(items) == 0 {
		log.Infow("nothing to fetch")
		return nil
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

			ok, attempts, bytesGot, alreadyOnDisk, lastErr := fetchAndPublish(ctx, client, pool, cfg, it)
			if !ok {
				log.Warnw("fetch failed",
					"height", it.entry.height, "attempts", attempts, "last_err", lastErr)
				recordRemaining(remainingW, remainingMu, it.entry.height, it.hashPath)
				counter.Lock()
				failed++
				counter.Unlock()
				return
			}
			counter.Lock()
			fetched++
			totalBytes += int64(bytesGot)
			snapBytes := totalBytes
			snapFetched := fetched
			counter.Unlock()
			runElapsed := time.Since(startedAt)
			log.Infow("fetched and published",
				"height", it.entry.height,
				"path", it.entry.localHash,
				"link", it.entry.localLink,
				"bytes", bytesGot,
				"already_on_disk", alreadyOnDisk,
				"attempts", attempts,
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

// mbPerSec returns throughput in decimal MB/s, rounded to 2 decimal places.
func mbPerSec(bytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	mb := float64(bytes) / 1_000_000.0
	rate := mb / elapsed.Seconds()
	return float64(int64(rate*100)) / 100
}

// fetchItem is one unit of work for shrex-fetch.
type fetchItem struct {
	entry    entry
	hashPath string // "blocks/<HASH>.ods"; for missing.remaining.txt formatting
}

// fetchAndPublish fetches the EDS bytes for the given height from peers,
// writes them to entry.localHash, and publishes the height link.
// Returns (success, attempts, bytesWritten, alreadyOnDisk, lastErr).
func fetchAndPublish(
	ctx context.Context,
	client *shrex.Client,
	pool *peerPool,
	cfg ShrexFetchConfig,
	it fetchItem,
) (bool, int, int, bool, error) {
	e := it.entry

	if e.empty {
		if err := publish(e); err != nil {
			return false, 0, 0, false, fmt.Errorf("publish empty: %w", err)
		}
		return true, 0, 0, true, nil
	}

	// Treat a block as already present only if it is store-readable (proper
	// ODS header + sibling .q4). A leftover raw-ODS file is NOT complete and
	// will be re-fetched and rewritten in the correct format below.
	blocksDir := filepath.Dir(e.localHash)
	if isStoreReadable(ctx, blocksDir, e.localHash) {
		if err := publish(e); err != nil {
			return false, 0, 0, false, fmt.Errorf("publish existing: %w", err)
		}
		return true, 0, 0, true, nil
	}

	// The store names blocks by DataHash; parse the expected hash from the
	// entry so we can reject a peer that returns data for the wrong square.
	var expected share.DataHash
	if hb, err := hex.DecodeString(extractHashFromPath(e.hashPath)); err == nil && len(hb) == share.DataHashSize {
		expected = share.DataHash(hb)
	}

	edsID, err := shwap.NewEdsID(e.height)
	if err != nil {
		return false, 0, 0, false, fmt.Errorf("new eds id: %w", err)
	}

	tried := make(map[peer.ID]struct{})
	var lastErr error
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return false, attempt, 0, false, err
		}
		p, ok := pool.pickExcluding(tried)
		if !ok {
			return false, attempt, 0, false, lastErr
		}
		tried[p] = struct{}{}

		reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		buff := bytes.NewBuffer(nil)
		getStart := time.Now()
		err := client.Get(reqCtx, &edsID, buff, p)
		cancel()
		if err != nil {
			lastErr = err
			log.Infow("shrex get failed",
				"height", e.height, "peer", p,
				"elapsed", time.Since(getStart).Round(time.Millisecond),
				"err", err)
			continue
		}

		// shrex returns raw ODS shares; rebuild and write the store ODSQ4 pair
		// (<hash>.ods + <hash>.q4). This also verifies the content hash.
		bytesGot := buff.Len()
		if _, err := encodeRawSharesAsODSQ4(blocksDir, buff.Bytes(), expected, true); err != nil {
			lastErr = fmt.Errorf("encode odsq4: %w", err)
			log.Warnw("encode failed", "height", e.height, "path", e.localHash, "err", err)
			continue
		}
		if err := publish(e); err != nil {
			lastErr = fmt.Errorf("publish: %w", err)
			log.Warnw("publish failed", "height", e.height, "err", err)
			continue
		}
		return true, attempt + 1, bytesGot, false, nil
	}
}

// buildItems prepares the list of heights to fetch with their full entries.
// Returns (items, missingPath) — missingPath is "" in range mode.
func buildItems(
	ctx context.Context,
	h host.Host,
	cfg ShrexFetchConfig,
	pool *peerPool,
) ([]fetchItem, string, error) {
	if cfg.FromHeight != 0 && cfg.ToHeight != 0 {
		items, err := buildItemsRange(ctx, h, cfg, pool)
		return items, "", err
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

// buildItemsRange fetches headers for [from..to] from the peer pool via
// libhead's p2p exchange and converts them into fetchItems.
func buildItemsRange(
	ctx context.Context,
	h host.Host,
	cfg ShrexFetchConfig,
	pool *peerPool,
) ([]fetchItem, error) {
	peers := pool.snapshot()
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers available for header exchange")
	}
	log.Infow("starting header exchange for range",
		"peers", len(peers), "from", cfg.FromHeight, "to", cfg.ToHeight)

	exchange, err := libhead_p2p.NewExchange[*header.ExtendedHeader](
		h, peers, nil,
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

	anchorReqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	anchor, err := exchange.GetByHeight(anchorReqCtx, cfg.FromHeight-1)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("anchor %d: %w", cfg.FromHeight-1, err)
	}

	const chunk = uint64(64)
	hdrs := make([]*header.ExtendedHeader, 0, cfg.ToHeight-cfg.FromHeight+1)
	for cur := cfg.FromHeight; cur <= cfg.ToHeight; {
		toExcl := cur + chunk
		if toExcl > cfg.ToHeight+1 {
			toExcl = cfg.ToHeight + 1
		}
		reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		batch, err := exchange.GetRangeByHeight(reqCtx, anchor, toExcl)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("get range %d..%d: %w", cur, toExcl-1, err)
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("empty header batch at %d", cur)
		}
		hdrs = append(hdrs, batch...)
		anchor = batch[len(batch)-1]
		cur = toExcl
		log.Infow("fetched header chunk", "from", batch[0].Height(), "to", anchor.Height())
	}

	items := make([]fetchItem, 0, len(hdrs))
	for _, hdr := range hdrs {
		hash := share.DataHash(hdr.DAH.Hash())
		items = append(items, fetchItem{
			entry:    entryFromHash(cfg.DataDir, hdr.Height(), hash),
			hashPath: "blocks/" + hash.String() + ".ods",
		})
	}
	return items, nil
}

// entryFromHash builds a replicate-compatible entry from a height and hash.
func entryFromHash(dataDir string, height uint64, hash share.DataHash) entry {
	hex := hash.String()
	return entry{
		height:     height,
		hashPath:   "blocks/" + hex + ".ods",
		localHash:  filepath.Join(dataDir, "blocks", hex+".ods"),
		localLink:  filepath.Join(dataDir, "blocks", "heights", fmt.Sprintf("%d.ods", height)),
		linkTarget: "../" + hex + ".ods",
		empty:      hash.IsEmptyEDS(),
	}
}

// readMissingItems parses missing.txt lines (`<height>\t<hashPath>`) into fetchItems.
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
		hashHex := extractHashFromPath(fields[1])
		if hashHex == "" {
			log.Warnw("malformed hashPath; skipping", "line", line)
			continue
		}
		hashBytes, err := hex.DecodeString(hashHex)
		if err != nil || len(hashBytes) != share.DataHashSize {
			log.Warnw("malformed hash; skipping", "hash", hashHex, "err", err)
			continue
		}
		seen[h] = struct{}{}
		items = append(items, fetchItem{
			entry:    entryFromHash(dataDir, h, share.DataHash(hashBytes)),
			hashPath: fields[1],
		})
	}
	return items, sc.Err()
}

// extractHashFromPath turns "blocks/ABC123.ods" into "ABC123". Returns "" if
// the path doesn't match that shape.
func extractHashFromPath(p string) string {
	base := filepath.Base(p)
	if !strings.HasSuffix(base, ".ods") {
		return ""
	}
	return strings.TrimSuffix(base, ".ods")
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

// peerPool is a thread-safe set of peer IDs for round-robin selection.
type peerPool struct {
	mu     sync.Mutex
	peers  []peer.ID
	notify []chan struct{}
}

func newPeerPool() *peerPool { return &peerPool{} }

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

func (p *peerPool) snapshot() peer.IDSlice {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(peer.IDSlice, len(p.peers))
	copy(out, p.peers)
	return out
}

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
