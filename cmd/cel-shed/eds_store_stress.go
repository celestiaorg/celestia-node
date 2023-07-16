package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/libs/edssser"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	edsStorePathFlag   = "eds-store"
	edsWritesFlag      = "eds-writes"
	edsSizeFlag        = "eds-size"
	edsEnableLogFlag   = "log"
	edsLogStatFreqFlag = "log-stat-freq"
	edsStoreCleanup    = "cleanup"
)

func init() {
	shareCmd.AddCommand(edsStoreStress)

	edsStoreStress.Flags().String(edsStorePathFlag, "", "Directory path to use for stress test. os.TempDir by default.")
	edsStoreStress.Flags().Int(edsWritesFlag, math.MaxInt, "Total EDS writes to make. MaxInt by default.")
	edsStoreStress.Flags().Int(edsSizeFlag, 128, "Chooses EDS size. 128 by default.")
	edsStoreStress.Flags().Bool(edsEnableLogFlag, true, "Enables logging. Disabled by default.")
	edsStoreStress.Flags().Int(edsLogStatFreqFlag, 10, "Write statistic logging frequency. 10 by default.")
	edsStoreStress.Flags().Bool(edsStoreCleanup, true, "Cleans up the store on stop. Enabled by default.")

	// kill redundant print
	nodebuilder.PrintKeyringInfo = false
}

var shareCmd = &cobra.Command{
	Use:   "share [subcommand]",
	Short: "Collection of share module related utilities",
}

var edsStoreStress = &cobra.Command{
	Use:          "eds-store-stress",
	Short:        `Runs eds.Store stress test over default node.Store Datastore backend (e.g. Badger).`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		_ = logging.SetLogLevel("*", "fatal")
		path, _ := cmd.Flags().GetString(edsStorePathFlag)
		if path == "" {
			path, err = os.MkdirTemp("", "*")
			if err != nil {
				return err
			}
		}
		fmt.Printf("Using %s as path for the EDS Store\n", path)

		cleanup, _ := cmd.Flags().GetBool(edsStoreCleanup)
		if cleanup {
			defer func() {
				err = errors.Join(err, os.RemoveAll(path))
			}()
		}

		enableLog, _ := cmd.Flags().GetBool(edsEnableLogFlag)
		logFreq, _ := cmd.Flags().GetInt(edsLogStatFreqFlag)
		edsWrites, _ := cmd.Flags().GetInt(edsWritesFlag)
		edsSize, _ := cmd.Flags().GetInt(edsSizeFlag)
		cfg := edssser.Config{
			EDSSize:     edsSize,
			EDSWrites:   edsWrites,
			EnableLog:   enableLog,
			StatLogFreq: logFreq,
		}

		err = nodebuilder.Init(*nodebuilder.DefaultConfig(node.Full), path, node.Full)
		if err != nil {
			return err
		}

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, nodestore.Close())
		}()

		datastore, err := nodestore.Datastore()
		if err != nil {
			return err
		}

		stresser, err := edssser.NewEDSsser(path, datastore, cfg)
		if err != nil {
			return err
		}

		stats, err := stresser.Run(cmd.Context())
		if !errors.Is(err, context.Canceled) {
			return err
		}

		fmt.Printf("%s", stats.Finalize())
		return nil
	},
}
