package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/libs/edssser"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	edsStorePathFlag   = "path"
	edsWritesFlag      = "writes"
	edsSizeFlag        = "size"
	edsDisableLogFlag  = "disable-log"
	edsLogStatFreqFlag = "log-stat-freq"
	edsCleanupFlag     = "cleanup"
	edsFreshStartFlag  = "fresh"
)

func init() {
	edsStoreCmd.AddCommand(edsStoreStress)

	defaultPath := os.TempDir() + "/eds-store"
	pathFlagUsage := fmt.Sprintf("Directory path to use for stress test. Uses %s by default.", defaultPath)
	edsStoreStress.Flags().String(edsStorePathFlag, defaultPath, pathFlagUsage)
	edsStoreStress.Flags().Int(edsWritesFlag, math.MaxInt, "Total EDS writes to make. MaxInt by default.")
	edsStoreStress.Flags().Int(edsSizeFlag, 128, "Chooses EDS size. 128 by default.")
	edsStoreStress.Flags().Bool(edsDisableLogFlag, false, "Disables logging. Enabled by default.")
	edsStoreStress.Flags().Int(edsLogStatFreqFlag, 10, "Write statistic logging frequency. 10 by default.")
	edsStoreStress.Flags().Bool(edsCleanupFlag, false, "Cleans up the store on stop. Disabled by default.")
	edsStoreStress.Flags().Bool(edsFreshStartFlag, false, "Cleanup previous state on start. Disabled by default.")

	// kill redundant print
	nodebuilder.PrintKeyringInfo = false
}

var edsStoreCmd = &cobra.Command{
	Use:   "eds-store [subcommand]",
	Short: "Collection of eds-store related utilities",
}

var edsStoreStress = &cobra.Command{
	Use:          "stress",
	Short:        `Runs eds.Store stress test over default node.Store Datastore backend (e.g. Badger).`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		path, _ := cmd.Flags().GetString(edsStorePathFlag)
		fmt.Printf("using %s as path for the EDS Store\n", path)

		freshStart, _ := cmd.Flags().GetBool(edsFreshStartFlag)
		if freshStart {
			err = os.RemoveAll(path)
			if err != nil {
				return err
			}
		}

		cleanup, _ := cmd.Flags().GetBool(edsCleanupFlag)
		if cleanup {
			defer func() {
				err = errors.Join(err, os.RemoveAll(path))
			}()
		}

		disableLog, _ := cmd.Flags().GetBool(edsDisableLogFlag)
		logFreq, _ := cmd.Flags().GetInt(edsLogStatFreqFlag)
		edsWrites, _ := cmd.Flags().GetInt(edsWritesFlag)
		edsSize, _ := cmd.Flags().GetInt(edsSizeFlag)
		cfg := edssser.Config{
			EDSSize:     edsSize,
			EDSWrites:   edsWrites,
			EnableLog:   !disableLog,
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
