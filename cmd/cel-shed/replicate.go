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
	flagRemoteHost     = "remote-host"
	flagRemoteBlocks   = "remote-blocks"
	flagSource         = "source"
	flagDataDir        = "data-dir"
	flagNetwork        = "network"
	flagFromHeight     = "from-height"
	flagToHeight       = "to-height"
	flagScanFromHeight = "scan-from-height"
	flagBatchSize      = "batch-size"
	flagLogLevel       = "log-level"
	flagConcurrency    = "concurrency"
	flagRequestTimeout = "request-timeout"
	flagVerify         = "verify"
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
