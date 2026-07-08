package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

const (
	flagRemoteHost       = "remote-host"
	flagRemoteBlocks     = "remote-blocks"
	flagSource           = "source"
	flagDataDir          = "data-dir"
	flagNetwork          = "network"
	flagFromHeight       = "from-height"
	flagToHeight         = "to-height"
	flagScanFromHeight   = "scan-from-height"
	flagBatchSize        = "batch-size"
	flagLogLevel         = "log-level"
	flagConcurrency      = "concurrency"
	flagRequestTimeout   = "request-timeout"
	flagVerify           = "verify"
	flagMissingFile      = "missing-file"
	flagPeers            = "peers"
	flagMinPeers         = "min-peers"
	flagDiscoveryTimeout = "discovery-timeout"
	flagDiscoveryLimit   = "discovery-limit"
	flagTempDir          = "temp-dir"
	flagSkipDownload     = "skip-download"
	flagRepair           = "repair"
	flagJSON             = "json"
)

func init() {
	replicateCmd.Flags().String(flagRemoteHost, "",
		"ssh-style remote host for rsync, e.g. ubuntu@da01-lunar")
	replicateCmd.Flags().String(flagRemoteBlocks, "",
		"absolute path of the remote blocks directory, e.g. /home/ubuntu/.celestia-bridge/blocks")
	replicateCmd.Flags().String(flagSource, "",
		"libp2p multiaddr of source bridge node, must include /p2p/<peer-id>")
	replicateCmd.Flags().String(flagDataDir, "",
		"destination data directory; must already be initialised via `celestia bridge init` (required)")
	replicateCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	replicateCmd.Flags().Uint64(flagFromHeight, 0,
		"header sync start height; 0 means resume from stored header head + 1")
	replicateCmd.Flags().Uint64(flagToHeight, 0,
		"stop height (inclusive); 0 means source's current head at start")
	replicateCmd.Flags().Uint64(flagScanFromHeight, 0,
		"block scan start height; 0 means scan from genesis for first missing block link")
	replicateCmd.Flags().Uint64(flagBatchSize, replicate.DefaultBatchSize,
		"number of heights per sequential rsync batch")
	replicateCmd.Flags().String(flagLogLevel, "warn",
		"log level for cel-shed/replicate logger")
	replicateCmd.Flags().Int(flagConcurrency, 8,
		"number of concurrent header range requests (1..32)")
	replicateCmd.Flags().Duration(flagRequestTimeout, 30*time.Second,
		"timeout for each header request attempt")
	replicateCmd.Flags().Bool(flagVerify, false,
		"after replication, verify copied block height links in the selected range")

	_ = replicateCmd.MarkFlagRequired(flagDataDir)
}

var replicateCmd = &cobra.Command{
	Use:          "replicate",
	Short:        "Replicate a Celestia bridge with simple sequential header + rsync batches.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readReplicateFlags(cmd)
		if err != nil {
			return err
		}
		return replicate.Run(cmd.Context(), cfg)
	},
}

func init() {
	replicateCmd.AddCommand(getMissingCmd, shrexFetchCmd, stagedSyncCmd, convertCmd, discoverArchivalCmd)
}

var discoverArchivalCmd = &cobra.Command{
	Use:   "discover-archival",
	Short: "Discover archival nodes advertising on the network's DHT.",
	Long: "Boots an ephemeral libp2p host, joins the network's Kademlia DHT, and " +
		"listens on the \"archival\" discovery topic for --timeout, then prints every " +
		"archival peer seen (as dialable multiaddrs). Read-only: advertises nothing and " +
		"fetches no data.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readDiscoverArchivalFlags(cmd)
		if err != nil {
			return err
		}
		return replicate.RunDiscoverArchival(cmd.Context(), cfg)
	},
}

func init() {
	discoverArchivalCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	discoverArchivalCmd.Flags().Duration(flagDiscoveryTimeout, 30*time.Second,
		"how long to keep discovering before printing results")
	discoverArchivalCmd.Flags().Uint(flagDiscoveryLimit, 100,
		"soft cap for the archival peer pool maintained by discovery")
	discoverArchivalCmd.Flags().Bool(flagJSON, false,
		"emit results as JSON instead of a text table")
	discoverArchivalCmd.Flags().String(flagLogLevel, "info",
		"log level for cel-shed/replicate logger")
}

func readDiscoverArchivalFlags(cmd *cobra.Command) (replicate.DiscoverConfig, error) {
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	timeout, _ := cmd.Flags().GetDuration(flagDiscoveryTimeout)
	discLimit, _ := cmd.Flags().GetUint(flagDiscoveryLimit)
	asJSON, _ := cmd.Flags().GetBool(flagJSON)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicate.DiscoverConfig{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicate.DiscoverConfig{
		Network:        net,
		Timeout:        timeout,
		DiscoveryLimit: discLimit,
		JSON:           asJSON,
		LogLevel:       logLevel,
	}, nil
}

var shrexFetchCmd = &cobra.Command{
	Use:   "shrex-fetch",
	Short: "Fetch ODS files for heights in missing.txt via shrex (archival peers).",
	Long: "Reads missing.txt, discovers archival peers via DHT (or uses --peers), and " +
		"fetches each height's ODS via shrex. Persists to blocks/<hash>.ods and publishes " +
		"the height link. Heights that fail are written to missing.remaining.txt.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readShrexFetchFlags(cmd)
		if err != nil {
			return err
		}
		return replicate.RunShrexFetch(cmd.Context(), cfg)
	},
}

func init() {
	shrexFetchCmd.Flags().String(flagDataDir, "",
		"destination data directory; must already be initialised via `celestia bridge init` (required)")
	shrexFetchCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	shrexFetchCmd.Flags().String(flagMissingFile, "",
		"path to missing.txt; empty = <data-dir>/.cel-shed-replicate/missing.txt (mutually exclusive with --from-height/--to-height)")
	shrexFetchCmd.Flags().Uint64(flagFromHeight, 0,
		"range mode: start height (inclusive); requires --to-height; mutually exclusive with --missing-file")
	shrexFetchCmd.Flags().Uint64(flagToHeight, 0,
		"range mode: stop height (inclusive); requires --from-height; mutually exclusive with --missing-file")
	shrexFetchCmd.Flags().StringSlice(flagPeers, nil,
		"explicit shrex peer multiaddrs (each must include /p2p/<peer-id>); if empty, discover archival peers via DHT")
	shrexFetchCmd.Flags().Int(flagConcurrency, 4,
		"number of concurrent fetches (1..32)")
	shrexFetchCmd.Flags().Duration(flagRequestTimeout, 90*time.Second,
		"timeout for each shrex request attempt")
	shrexFetchCmd.Flags().Int(flagMinPeers, 3,
		"minimum archival peers to wait for before starting fetches (discovery mode)")
	shrexFetchCmd.Flags().Duration(flagDiscoveryTimeout, 60*time.Second,
		"max time to wait for min-peers archival peers before failing")
	shrexFetchCmd.Flags().Uint(flagDiscoveryLimit, 20,
		"soft cap for the archival peer pool maintained by discovery")
	shrexFetchCmd.Flags().String(flagLogLevel, "info",
		"log level for cel-shed/replicate logger")
	_ = shrexFetchCmd.MarkFlagRequired(flagDataDir)
}

func readShrexFetchFlags(cmd *cobra.Command) (replicate.ShrexFetchConfig, error) {
	dataDir, _ := cmd.Flags().GetString(flagDataDir)
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	missingFile, _ := cmd.Flags().GetString(flagMissingFile)
	fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
	toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
	peers, _ := cmd.Flags().GetStringSlice(flagPeers)
	concurrency, _ := cmd.Flags().GetInt(flagConcurrency)
	reqTimeout, _ := cmd.Flags().GetDuration(flagRequestTimeout)
	minPeers, _ := cmd.Flags().GetInt(flagMinPeers)
	discTimeout, _ := cmd.Flags().GetDuration(flagDiscoveryTimeout)
	discLimit, _ := cmd.Flags().GetUint(flagDiscoveryLimit)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)

	expanded, err := homedir.Expand(filepath.Clean(dataDir))
	if err != nil {
		return replicate.ShrexFetchConfig{}, fmt.Errorf("expand --data-dir: %w", err)
	}

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicate.ShrexFetchConfig{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicate.ShrexFetchConfig{
		DataDir:          expanded,
		Network:          net,
		MissingFile:      missingFile,
		FromHeight:       fromHeight,
		ToHeight:         toHeight,
		Peers:            peers,
		Concurrency:      concurrency,
		RequestTimeout:   reqTimeout,
		MinPeers:         minPeers,
		DiscoveryTimeout: discTimeout,
		DiscoveryLimit:   discLimit,
		LogLevel:         logLevel,
	}, nil
}

var getMissingCmd = &cobra.Command{
	Use:   "get-missing",
	Short: "Scan blocks/heights/ for gaps and reconcile against header store + blocks/.",
	Long: "Scans heights/ for present block links, writes sorted present.txt and gaps.txt, " +
		"then reconciles each gap: if the ODS file exists in blocks/, publishes the height link; " +
		"otherwise appends the height to missing.txt for later fetching.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readGetMissingFlags(cmd)
		if err != nil {
			return err
		}
		return replicate.RunGetMissing(cmd.Context(), cfg)
	},
}

func init() {
	getMissingCmd.Flags().String(flagDataDir, "",
		"destination data directory; must already be initialised via `celestia bridge init` (required)")
	getMissingCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	getMissingCmd.Flags().Uint64(flagFromHeight, 0,
		"scan start height (inclusive); 0 means 1")
	getMissingCmd.Flags().Uint64(flagToHeight, 0,
		"scan stop height (inclusive); 0 means header store head")
	getMissingCmd.Flags().String(flagLogLevel, "info",
		"log level for cel-shed/replicate logger")
	_ = getMissingCmd.MarkFlagRequired(flagDataDir)
}

func readGetMissingFlags(cmd *cobra.Command) (replicate.GetMissingConfig, error) {
	dataDir, _ := cmd.Flags().GetString(flagDataDir)
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
	toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)

	expanded, err := homedir.Expand(filepath.Clean(dataDir))
	if err != nil {
		return replicate.GetMissingConfig{}, fmt.Errorf("expand --data-dir: %w", err)
	}

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicate.GetMissingConfig{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicate.GetMissingConfig{
		DataDir:    expanded,
		Network:    net,
		FromHeight: fromHeight,
		ToHeight:   toHeight,
		LogLevel:   logLevel,
	}, nil
}

var stagedSyncCmd = &cobra.Command{
	Use:   "staged-sync",
	Short: "Fill heights/ gaps via an isolated temp staging dir, then copy into place.",
	Long: "Scans <data-dir>/blocks/heights for internal gaps, downloads the missing " +
		"headers into a badger DB under <temp-dir>/headers (plus a manifest) and their " +
		"ODS via shrex into <temp-dir>/blocks, then copies each ODS into " +
		"<data-dir>/blocks/<HASH>.ods and symlinks <data-dir>/blocks/heights/<height>.ods. " +
		"Use --skip-download to replay only the copy step from an existing staging dir.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readStagedSyncFlags(cmd)
		if err != nil {
			return err
		}
		return replicate.RunStagedSync(cmd.Context(), cfg)
	},
}

func init() {
	stagedSyncCmd.Flags().String(flagDataDir, "",
		"destination data directory containing blocks/heights (required)")
	stagedSyncCmd.Flags().String(flagTempDir, "",
		"staging directory for the header DB and downloaded ODS; must differ from --data-dir (required)")
	stagedSyncCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	stagedSyncCmd.Flags().Uint64(flagFromHeight, 0,
		"gap-scan start height (inclusive); 0 means the lowest height present in heights/")
	stagedSyncCmd.Flags().Uint64(flagToHeight, 0,
		"gap-scan stop height (inclusive); 0 means the highest height present in heights/")
	stagedSyncCmd.Flags().StringSlice(flagPeers, nil,
		"explicit shrex peer multiaddrs (each must include /p2p/<peer-id>); if empty, discover archival peers via DHT")
	stagedSyncCmd.Flags().Int(flagConcurrency, 8,
		"number of concurrent ODS fetches (1..32)")
	stagedSyncCmd.Flags().Duration(flagRequestTimeout, 90*time.Second,
		"timeout for each shrex/header request attempt")
	stagedSyncCmd.Flags().Int(flagMinPeers, 3,
		"minimum archival peers to wait for before starting (discovery mode)")
	stagedSyncCmd.Flags().Duration(flagDiscoveryTimeout, 60*time.Second,
		"max time to wait for min-peers archival peers before failing")
	stagedSyncCmd.Flags().Uint(flagDiscoveryLimit, 20,
		"soft cap for the archival peer pool maintained by discovery")
	stagedSyncCmd.Flags().Bool(flagSkipDownload, false,
		"skip the download phase; copy already-staged blocks from --temp-dir into place")
	stagedSyncCmd.Flags().Bool(flagRepair, false,
		"repair mode: also re-fetch present heights whose block is unreadable by the store (fails OpenODS or lacks a .q4)")
	stagedSyncCmd.Flags().String(flagLogLevel, "info",
		"log level for cel-shed/replicate logger")
	_ = stagedSyncCmd.MarkFlagRequired(flagDataDir)
	_ = stagedSyncCmd.MarkFlagRequired(flagTempDir)
}

func readStagedSyncFlags(cmd *cobra.Command) (replicate.StagedSyncConfig, error) {
	dataDir, _ := cmd.Flags().GetString(flagDataDir)
	tempDir, _ := cmd.Flags().GetString(flagTempDir)
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
	toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
	peers, _ := cmd.Flags().GetStringSlice(flagPeers)
	concurrency, _ := cmd.Flags().GetInt(flagConcurrency)
	reqTimeout, _ := cmd.Flags().GetDuration(flagRequestTimeout)
	minPeers, _ := cmd.Flags().GetInt(flagMinPeers)
	discTimeout, _ := cmd.Flags().GetDuration(flagDiscoveryTimeout)
	discLimit, _ := cmd.Flags().GetUint(flagDiscoveryLimit)
	skipDownload, _ := cmd.Flags().GetBool(flagSkipDownload)
	repair, _ := cmd.Flags().GetBool(flagRepair)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)

	dataExpanded, err := homedir.Expand(filepath.Clean(dataDir))
	if err != nil {
		return replicate.StagedSyncConfig{}, fmt.Errorf("expand --data-dir: %w", err)
	}
	tempExpanded, err := homedir.Expand(filepath.Clean(tempDir))
	if err != nil {
		return replicate.StagedSyncConfig{}, fmt.Errorf("expand --temp-dir: %w", err)
	}

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicate.StagedSyncConfig{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicate.StagedSyncConfig{
		DataDir:          dataExpanded,
		TempDir:          tempExpanded,
		Network:          net,
		FromHeight:       fromHeight,
		ToHeight:         toHeight,
		Peers:            peers,
		Concurrency:      concurrency,
		RequestTimeout:   reqTimeout,
		MinPeers:         minPeers,
		DiscoveryTimeout: discTimeout,
		DiscoveryLimit:   discLimit,
		SkipDownload:     skipDownload,
		Repair:           repair,
		LogLevel:         logLevel,
	}, nil
}

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Offline: re-encode present raw-ODS blocks into the store's ODSQ4 format in place.",
	Long: "Scans <data-dir>/blocks/heights and, for each present block that is not " +
		"store-readable (old raw-ODS format, no .q4), reads the on-disk shares, rebuilds " +
		"the square, recomputes the hash locally, and rewrites it as a proper <hash>.ods + " +
		"<hash>.q4 pair with the height link repointed at it. No network is used. Blocks " +
		"already in store format are skipped; absent or corrupt heights are reported for " +
		"`staged-sync --repair`.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		dataDir, _ := cmd.Flags().GetString(flagDataDir)
		fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
		toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
		logLevel, _ := cmd.Flags().GetString(flagLogLevel)
		expanded, err := homedir.Expand(filepath.Clean(dataDir))
		if err != nil {
			return fmt.Errorf("expand --data-dir: %w", err)
		}
		return replicate.RunConvert(cmd.Context(), replicate.ConvertConfig{
			DataDir:    expanded,
			FromHeight: fromHeight,
			ToHeight:   toHeight,
			LogLevel:   logLevel,
		})
	},
}

func init() {
	convertCmd.Flags().String(flagDataDir, "",
		"data directory containing blocks/heights (required)")
	convertCmd.Flags().Uint64(flagFromHeight, 0,
		"start height (inclusive); 0 means the lowest height present in heights/")
	convertCmd.Flags().Uint64(flagToHeight, 0,
		"stop height (inclusive); 0 means the highest height present in heights/")
	convertCmd.Flags().String(flagLogLevel, "info",
		"log level for cel-shed/replicate logger")
	_ = convertCmd.MarkFlagRequired(flagDataDir)
}

func readReplicateFlags(cmd *cobra.Command) (replicate.Config, error) {
	remoteHost, _ := cmd.Flags().GetString(flagRemoteHost)
	remoteBlocks, _ := cmd.Flags().GetString(flagRemoteBlocks)
	source, _ := cmd.Flags().GetString(flagSource)
	dataDir, _ := cmd.Flags().GetString(flagDataDir)
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
	toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
	scanFromHeight, _ := cmd.Flags().GetUint64(flagScanFromHeight)
	batchSize, _ := cmd.Flags().GetUint64(flagBatchSize)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)
	concurrency, _ := cmd.Flags().GetInt(flagConcurrency)
	reqTimeout, _ := cmd.Flags().GetDuration(flagRequestTimeout)
	verify, _ := cmd.Flags().GetBool(flagVerify)

	expanded, err := homedir.Expand(filepath.Clean(dataDir))
	if err != nil {
		return replicate.Config{}, fmt.Errorf("expand --data-dir: %w", err)
	}

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicate.Config{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicate.Config{
		RemoteHost:     remoteHost,
		RemoteBlocks:   remoteBlocks,
		Source:         source,
		DataDir:        expanded,
		Network:        net,
		FromHeight:     fromHeight,
		ToHeight:       toHeight,
		ScanFromHeight: scanFromHeight,
		BatchSize:      batchSize,
		Concurrency:    concurrency,
		RequestTimeout: reqTimeout,
		LogLevel:       logLevel,
		Verify:         verify,
	}, nil
}
