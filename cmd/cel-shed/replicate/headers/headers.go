package headers

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	libhead_p2p "github.com/celestiaorg/go-header/p2p"
	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var log = logging.Logger("cmd-shed/replicate/headers")

const ProgressInterval = 10 * time.Second

func Run(ctx context.Context, cfg Config, prog *Progress) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if prog == nil {
		prog = NewProgress()
	}

	if !nodebuilder.IsInit(cfg.DataDir) {
		return fmt.Errorf(
			"data directory %q is not initialised; run `celestia bridge init --node.store %s --p2p.network %s` first",
			cfg.DataDir, cfg.DataDir, cfg.Network,
		)
	}

	log.Infow("opening node store", "data_dir", cfg.DataDir)
	openStart := time.Now()
	nodeStore, err := nodebuilder.OpenStore(cfg.DataDir, nil)
	if err != nil {
		return fmt.Errorf("open node store: %w", err)
	}
	defer nodeStore.Close()
	ds, err := nodeStore.Datastore()
	if err != nil {
		return fmt.Errorf("open datastore: %w", err)
	}
	log.Infow("opened node store", "elapsed", time.Since(openStart).Round(time.Second))

	h, err := NewReplicatorHost()
	if err != nil {
		return fmt.Errorf("new libp2p host: %w", err)
	}
	defer h.Close()

	srcInfo, err := peer.AddrInfoFromString(cfg.Source)
	if err != nil {
		return fmt.Errorf("parse source multiaddr: %w", err)
	}
	h.Peerstore().AddAddrs(srcInfo.ID, srcInfo.Addrs, peerstore.PermanentAddrTTL)
	h.ConnManager().Protect(srcInfo.ID, "replicate-headers-source")

	log.Infow("dialing source", "peer", srcInfo.ID)
	dialStart := time.Now()
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	if err := h.Connect(connectCtx, *srcInfo); err != nil {
		connectCancel()
		return fmt.Errorf("connect to source %s: %w", srcInfo.ID, err)
	}
	connectCancel()
	log.Infow("connected to source", "elapsed", time.Since(dialStart).Round(time.Second))

	exchange, err := libhead_p2p.NewExchange[*header.ExtendedHeader](
		h,
		peer.IDSlice{srcInfo.ID},
		nil,
		libhead_p2p.WithNetworkID[libhead_p2p.ClientParameters](cfg.Network.String()),
		libhead_p2p.WithChainID(cfg.Network.String()),
	)
	if err != nil {
		return fmt.Errorf("new header exchange: %w", err)
	}
	if err := exchange.Start(ctx); err != nil {
		return fmt.Errorf("start header exchange: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exchange.Stop(stopCtx)
	}()

	log.Infow("starting header store")
	hstoreStart := time.Now()
	hstore, err := libhead_store.NewStore[*header.ExtendedHeader](
		ds,
		libhead_store.WithWriteBatchSize(1024),
	)
	if err != nil {
		return fmt.Errorf("new header store: %w", err)
	}
	if err := hstore.Start(ctx); err != nil {
		return fmt.Errorf("start header store: %w", err)
	}
	log.Infow("header store started", "elapsed", time.Since(hstoreStart).Round(time.Second))
	hstoreStopped := false
	stopHStore := func() error {
		if hstoreStopped {
			return nil
		}
		hstoreStopped = true
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := hstore.Stop(stopCtx); err != nil {
			return fmt.Errorf("stop header store: %w", err)
		}
		return nil
	}
	defer stopHStore()

	log.Infow("fetching source head")
	headStart := time.Now()
	srcHead, err := exchange.Head(ctx)
	if err != nil {
		return fmt.Errorf("fetch source head: %w", err)
	}
	log.Infow("got source head", "height", srcHead.Height(), "elapsed", time.Since(headStart).Round(time.Second))
	if srcHead.ChainID() != cfg.Network.String() {
		return fmt.Errorf("network mismatch: network=%s but source chain-id=%s",
			cfg.Network, srcHead.ChainID())
	}

	startHeight, targetHeight, err := resolveRange(ctx, hstore, cfg.FromHeight, cfg.ToHeight, srcHead.Height())
	if err != nil {
		return err
	}
	prog.SetTargetHeight(targetHeight)
	log.Infow("resolved header range", "start", startHeight, "target", targetHeight)
	if startHeight > targetHeight {
		log.Infow("headers already at or beyond target; nothing to fetch")
		return nil
	}

	prog.Init(targetHeight - startHeight + 1)

	tickerCtx, tickerCancel := context.WithCancel(ctx)
	defer tickerCancel()
	progressStart := time.Now()
	go func() {
		t := time.NewTicker(ProgressInterval)
		defer t.Stop()
		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-t.C:
				log.Infow("headers progress",
					"stored", prog.Stored(),
					"target", prog.Target(),
					"target_height", prog.TargetHeight(),
					"elapsed", time.Since(progressStart).Round(time.Second).String(),
				)
			}
		}
	}()

	lastAppended, replicateErr := replicateHeaderRange(
		ctx,
		exchange,
		hstore,
		startHeight,
		targetHeight,
		cfg.Concurrency,
		cfg.RequestTimeout,
		cfg.Network.String(),
		prog,
	)

	if err := stopHStore(); err != nil {
		return err
	}

	if replicateErr != nil {
		if lastAppended != nil {
			if err := PersistHeadKey(ds, lastAppended); err != nil {
				return fmt.Errorf("persist partial head: %w", err)
			}
		}
		if errors.Is(replicateErr, context.Canceled) {
			return nil
		}
		return replicateErr
	}

	if lastAppended == nil {
		return fmt.Errorf("internal error: completed non-empty replication without appending a header")
	}
	if lastAppended.Height() != targetHeight {
		return fmt.Errorf("internal error: appended to height %d, expected %d",
			lastAppended.Height(), targetHeight)
	}
	return PersistHeadKey(ds, lastAppended)
}
