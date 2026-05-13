package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	replicatesync "github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate-sync"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

const flagBatchSize = "batch-size"

func init() {
	replicateSyncCmd.Flags().String(flagRemoteHost, "",
		"ssh-style remote host for rsync, e.g. ubuntu@da01-lunar")
	replicateSyncCmd.Flags().String(flagRemoteBlocks, "",
		"absolute path of the remote blocks directory, e.g. /home/ubuntu/.celestia-bridge/blocks")
	replicateSyncCmd.Flags().String(flagSource, "",
		"libp2p multiaddr of source bridge node, must include /p2p/<peer-id>")
	replicateSyncCmd.Flags().String(flagDataDir, "",
		"destination data directory; must already be initialised via `celestia bridge init` (required)")
	replicateSyncCmd.Flags().String(flagNetwork, "mainnet",
		"network: mainnet | mocha | arabica | private")
	replicateSyncCmd.Flags().Uint64(flagFromHeight, 0,
		"start height; 0 means scan from genesis for first missing block link")
	replicateSyncCmd.Flags().Uint64(flagToHeight, 0,
		"stop height (inclusive); 0 means source's current head at start")
	replicateSyncCmd.Flags().Uint64(flagBatchSize, replicatesync.DefaultBatchSize,
		"number of heights per sequential rsync batch")
	replicateSyncCmd.Flags().String(flagLogLevel, "warn",
		"log level for cel-shed/replicate-sync logger")
	replicateSyncCmd.Flags().Int(flagConcurrency, 8,
		"number of concurrent header range requests (1..32)")
	replicateSyncCmd.Flags().Duration(flagRequestTimeout, 30*time.Second,
		"timeout for each header request attempt")
	replicateSyncCmd.Flags().Bool(flagVerify, false,
		"after replication, verify copied block height links in the selected range")

	_ = replicateSyncCmd.MarkFlagRequired(flagDataDir)
}

var replicateSyncCmd = &cobra.Command{
	Use:          "replicate-sync",
	Short:        "Replicate a Celestia bridge with simple sequential header + rsync batches.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := readReplicateSyncFlags(cmd)
		if err != nil {
			return err
		}
		return replicatesync.Run(cmd.Context(), cfg)
	},
}

func readReplicateSyncFlags(cmd *cobra.Command) (replicatesync.Config, error) {
	remoteHost, _ := cmd.Flags().GetString(flagRemoteHost)
	remoteBlocks, _ := cmd.Flags().GetString(flagRemoteBlocks)
	source, _ := cmd.Flags().GetString(flagSource)
	dataDir, _ := cmd.Flags().GetString(flagDataDir)
	networkStr, _ := cmd.Flags().GetString(flagNetwork)
	fromHeight, _ := cmd.Flags().GetUint64(flagFromHeight)
	toHeight, _ := cmd.Flags().GetUint64(flagToHeight)
	batchSize, _ := cmd.Flags().GetUint64(flagBatchSize)
	logLevel, _ := cmd.Flags().GetString(flagLogLevel)
	concurrency, _ := cmd.Flags().GetInt(flagConcurrency)
	reqTimeout, _ := cmd.Flags().GetDuration(flagRequestTimeout)
	verify, _ := cmd.Flags().GetBool(flagVerify)

	expanded, err := homedir.Expand(filepath.Clean(dataDir))
	if err != nil {
		return replicatesync.Config{}, fmt.Errorf("expand --data-dir: %w", err)
	}

	net, err := modp2p.GetNetwork(networkStr).Validate()
	if err != nil {
		net2, err2 := modp2p.Network(networkStr).Validate()
		if err2 != nil {
			return replicatesync.Config{}, fmt.Errorf("invalid --network %q: %w", networkStr, err)
		}
		net = net2
	}

	return replicatesync.Config{
		RemoteHost:     remoteHost,
		RemoteBlocks:   remoteBlocks,
		Source:         source,
		DataDir:        expanded,
		Network:        net,
		FromHeight:     fromHeight,
		ToHeight:       toHeight,
		BatchSize:      batchSize,
		Concurrency:    concurrency,
		RequestTimeout: reqTimeout,
		LogLevel:       logLevel,
		Verify:         verify,
	}, nil
}
