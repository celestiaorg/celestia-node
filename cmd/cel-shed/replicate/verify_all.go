package replicate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// VerifyAllConfig configures a combined integrity audit that runs BOTH the deep,
// offline ODS check (verify-ods) and the fast, chain-header check (verify-headers)
// over the same data directory and height range.
//
// The two checks are complementary:
//
//   - verify-ods recomputes each block's DataHash from the shares on disk, so it
//     catches silent corruption inside the data — but it has no notion of what the
//     chain actually committed.
//   - verify-headers downloads every header and compares the chain-committed
//     DataHash to the one stored in the ODS header (and the filename / hardlink),
//     so it proves the stored hash is the canonical one — but it never re-hashes
//     the shares, so it cannot see a flipped byte that leaves the header hash
//     unchanged.
//
// Running both proves, end to end, that the on-disk shares hash to the value in
// the ODS header AND that that value is the one the chain committed.
//
// When Parallel is set the two checks run concurrently: the CPU-bound verify-ods
// scan overlaps the (network-bound) header download and the light header check,
// so the wall-clock cost is close to the slower of the two rather than their sum.
// Both always run to completion — a failure in one does not stop the other, so a
// single run yields both verdicts. Each check writes its own failed file
// (verify-failed.txt and verify-headers-failed.txt), so there is no conflict.
type VerifyAllConfig struct {
	DataDir string
	// Source is the libp2p multiaddr of the bridge node headers are downloaded
	// from (required — the verify-headers half needs it).
	Source     string
	Network    modp2p.Network
	FromHeight uint64
	ToHeight   uint64
	FailFast   bool
	// Concurrency is the number of parallel verification workers used by EACH
	// check. 0 means one worker per CPU core.
	Concurrency int
	// HeaderConcurrency is the number of concurrent header range requests during
	// the download phase (1..32). 0 means 8.
	HeaderConcurrency int
	RequestTimeout    time.Duration
	// HeaderStoreDir is the standalone badger directory the downloaded headers are
	// written to; the node's own store under DataDir is never touched. Empty means
	// <data-dir>/.cel-shed-replicate/verify-headers-db.
	HeaderStoreDir string
	LogLevel       string
	// Parallel runs verify-ods and verify-headers concurrently instead of one
	// after the other.
	Parallel bool
}

func (c VerifyAllConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("source is required (the verify-headers check downloads headers from it)")
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

// odsConfig projects the shared settings onto the verify-ods check.
func (c VerifyAllConfig) odsConfig() VerifyConfig {
	return VerifyConfig{
		DataDir:     c.DataDir,
		FromHeight:  c.FromHeight,
		ToHeight:    c.ToHeight,
		FailFast:    c.FailFast,
		Concurrency: c.Concurrency,
		LogLevel:    c.LogLevel,
	}
}

// headersConfig projects the shared settings onto the verify-headers check.
func (c VerifyAllConfig) headersConfig() VerifyHeadersConfig {
	return VerifyHeadersConfig{
		DataDir:           c.DataDir,
		Source:            c.Source,
		Network:           c.Network,
		FromHeight:        c.FromHeight,
		ToHeight:          c.ToHeight,
		FailFast:          c.FailFast,
		Concurrency:       c.Concurrency,
		HeaderConcurrency: c.HeaderConcurrency,
		RequestTimeout:    c.RequestTimeout,
		HeaderStoreDir:    c.HeaderStoreDir,
		LogLevel:          c.LogLevel,
	}
}

// RunVerifyAll runs both verify-ods and verify-headers over the same range,
// sequentially or (with cfg.Parallel) concurrently. It returns a non-nil error if
// either check finds a verification failure; both checks always run to completion.
func RunVerifyAll(ctx context.Context, cfg VerifyAllConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
		_ = logging.SetLogLevel("cmd-shed/replicate/headers", cfg.LogLevel)
	}

	odsCfg := cfg.odsConfig()
	hdrCfg := cfg.headersConfig()

	return runBothChecks(ctx, cfg.Parallel,
		func(ctx context.Context) error { return RunVerify(ctx, odsCfg) },
		func(ctx context.Context) error { return RunVerifyHeaders(ctx, hdrCfg) },
	)
}

// runBothChecks orchestrates the ODS check (odsFn) and the headers check (hdrFn),
// either sequentially or concurrently, and returns the combined error (nil only
// if both pass). Both funcs always run to completion even when one reports
// failures, so a single invocation yields both verdicts. The check functions are
// injected so the orchestration can be unit-tested without a network download.
func runBothChecks(ctx context.Context, parallel bool, odsFn, hdrFn func(context.Context) error) error {
	start := time.Now()
	var odsErr, hdrErr error

	if parallel {
		log.Infow("verify-all: running verify-ods and verify-headers concurrently")
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			odsErr = odsFn(ctx)
		}()
		go func() {
			defer wg.Done()
			hdrErr = hdrFn(ctx)
		}()
		wg.Wait()
	} else {
		log.Infow("verify-all: running verify-ods, then verify-headers")
		odsErr = odsFn(ctx)
		hdrErr = hdrFn(ctx)
	}

	elapsed := time.Since(start).Round(time.Second)
	if odsErr == nil && hdrErr == nil {
		log.Infow("verify-all: PASSED both checks", "elapsed", elapsed)
		return nil
	}
	if odsErr != nil {
		log.Errorw("verify-all: verify-ods FAILED", "err", odsErr)
	}
	if hdrErr != nil {
		log.Errorw("verify-all: verify-headers FAILED", "err", hdrErr)
	}
	log.Warnw("verify-all: completed WITH FAILURES", "elapsed", elapsed,
		"ods_ok", odsErr == nil, "headers_ok", hdrErr == nil)
	return errors.Join(odsErr, hdrErr)
}
