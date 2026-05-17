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
	replicateCmd.AddCommand(getMissingCmd, shrexFetchCmd)
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
