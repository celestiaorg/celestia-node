package main

import (
	"context"
	"errors"
	_ "expvar"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/grafana/pyroscope-go"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/libs/edssser"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	edsStorePathFlag   = "path"
	edsWritesFlag      = "writes"
	edsWriteFrom       = "init_height"
	edsSizeFlag        = "size"
	edsDisableLogFlag  = "disable-log"
	edsLogStatFreqFlag = "log-stat-freq"
	edsCleanupFlag     = "cleanup"
	edsFreshStartFlag  = "fresh"

	pyroscopeEndpointFlag = "pyroscope"
	putTimeoutFlag        = "timeout"
	badgerLogLevelFlag    = "badger-log-level"
)

func init() {
	edsStoreCmd.AddCommand(edsStoreStress)

	defaultPath := "~/.edssser"
	path, err := homedir.Expand(defaultPath)
	if err != nil {
		panic(err)
	}

	pathFlagUsage := fmt.Sprintf("Directory path to use for stress test. Uses %s by default.", defaultPath)
	edsStoreStress.Flags().String(edsStorePathFlag, path, pathFlagUsage)
	edsStoreStress.Flags().String(pyroscopeEndpointFlag, "",
		"Pyroscope address. If no address provided, pyroscope will be disabled")
	edsStoreStress.Flags().Int(edsWritesFlag, math.MaxInt, "Total EDS writes to make. MaxInt by default.")
	edsStoreStress.Flags().Int(edsSizeFlag, 128, "Chooses EDS size. 128 by default.")
	edsStoreStress.Flags().Bool(edsDisableLogFlag, false, "Disables logging. Enabled by default.")
	edsStoreStress.Flags().Int(edsLogStatFreqFlag, 10, "Write statistic logging frequency. 10 by default.")
	edsStoreStress.Flags().Bool(edsCleanupFlag, false, "Cleans up the store on stop. Disabled by default.")
	edsStoreStress.Flags().Bool(edsFreshStartFlag, false, "Cleanup previous state on start. Disabled by default.")
	edsStoreStress.Flags().Int(putTimeoutFlag, 30, "Sets put timeout in seconds. 30 sec by default.")
	edsStoreStress.Flags().String(badgerLogLevelFlag, "INFO", "Badger log level, Defaults to INFO")

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
	RunE: func(cmd *cobra.Command, _ []string) (err error) {
		// expose expvar vars over http
		go http.ListenAndServe(":9999", http.DefaultServeMux) //nolint:errcheck,gosec

		endpoint, _ := cmd.Flags().GetString(pyroscopeEndpointFlag)
		if endpoint != "" {
			_, err = pyroscope.Start(pyroscope.Config{
				ApplicationName: "cel-shred.stresser",
				ServerAddress:   endpoint,
				ProfileTypes: []pyroscope.ProfileType{
					pyroscope.ProfileCPU,
					pyroscope.ProfileAllocObjects,
					pyroscope.ProfileAllocSpace,
					pyroscope.ProfileInuseObjects,
					pyroscope.ProfileInuseSpace,
				},
			})
			if err != nil {
				fmt.Printf("failed to launch pyroscope with addr: %s err: %s\n", endpoint, err.Error())
			} else {
				fmt.Println("connected pyroscope to:", endpoint)
			}
		}

		path, _ := cmd.Flags().GetString(edsStorePathFlag)
		fmt.Printf("using %s\n", path)

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

		loglevel, _ := cmd.Flags().GetString(badgerLogLevelFlag)
		if err = logging.SetLogLevel("badger", loglevel); err != nil {
			return err
		}

		disableLog, _ := cmd.Flags().GetBool(edsDisableLogFlag)
		logFreq, _ := cmd.Flags().GetInt(edsLogStatFreqFlag)
		edsWrites, _ := cmd.Flags().GetInt(edsWritesFlag)
		writeFrom, _ := cmd.Flags().GetInt(edsWriteFrom)
		edsSize, _ := cmd.Flags().GetInt(edsSizeFlag)
		putTimeout, _ := cmd.Flags().GetInt(putTimeoutFlag)

		cfg := edssser.Config{
			EDSSize:     edsSize,
			EDSWrites:   edsWrites,
			WriteFrom:   writeFrom,
			EnableLog:   !disableLog,
			LogFilePath: path,
			StatLogFreq: logFreq,
			OpTimeout:   time.Duration(putTimeout) * time.Second,
		}

		err = nodebuilder.Init(*nodebuilder.DefaultConfig(node.Bridge), path, node.Bridge)
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

		stresser, err := edssser.NewEDSsser(path, cfg)
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
