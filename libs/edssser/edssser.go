package edssser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

type Config struct {
	EDSSize     int
	EDSWrites   int
	EnableLog   bool
	LogFilePath string
	StatLogFreq int
}

// EDSsser stand for EDS Store Stresser.
type EDSsser struct {
	config     Config
	datastore  datastore.Batching
	edsstoreMu sync.Mutex
	edsstore   *eds.Store

	statsFileMu sync.Mutex
	statsFile   *os.File
}

func NewEDSsser(path string, datastore datastore.Batching, cfg Config) (*EDSsser, error) {
	edsstore, err := eds.NewStore(path, datastore)
	if err != nil {
		return nil, err
	}

	return &EDSsser{
		config:    cfg,
		datastore: datastore,
		edsstore:  edsstore,
	}, nil
}

func (ss *EDSsser) Run(ctx context.Context) (stats Stats, err error) {
	ss.edsstoreMu.Lock()
	defer ss.edsstoreMu.Unlock()

	err = ss.edsstore.Start(ctx)
	if err != nil {
		return stats, err
	}
	defer func() {
		err = errors.Join(err, ss.edsstore.Stop(ctx))
	}()

	edsHashes, err := ss.edsstore.List()
	if err != nil {
		return stats, err
	}
	fmt.Printf("recovered %d EDSes\n\n", len(edsHashes))

	defer func() {
		err = errors.Join(err, ss.dumpStat(stats.Finalize()))
	}()

	t := &testing.T{}
	for toWrite := ss.config.EDSWrites - len(edsHashes); ctx.Err() == nil && toWrite > 0; toWrite-- {
		// divide by 2 to get ODS size as expected by RandEDS
		square := edstest.RandEDS(t, ss.config.EDSSize/2)
		dah, err := da.NewDataAvailabilityHeader(square)
		if err != nil {
			return stats, err
		}

		now := time.Now()
		err = ss.edsstore.Put(ctx, dah.Hash(), square)
		if err != nil {
			return stats, err
		}
		took := time.Since(now)

		stats.TotalWritten++
		stats.TotalTime += took
		if took < stats.MinTime || stats.MinTime == 0 {
			stats.MinTime = took
		} else if took > stats.MaxTime {
			stats.MaxTime = took
		}

		if ss.config.EnableLog {
			fmt.Println("square written", "size", ss.config.EDSSize, "took", took)

			if stats.TotalWritten%ss.config.StatLogFreq == 0 {
				stats := stats.Finalize()
				fmt.Println(stats)
				go func() {
					err := ss.dumpStat(stats)
					if err != nil {
						fmt.Printf("error dumping stats: %s\n", err.Error())
					}
				}()
			}
		}
	}

	return stats, nil
}

func (ss *EDSsser) dumpStat(stats Stats) (err error) {
	ss.statsFileMu.Lock()
	defer ss.statsFileMu.Unlock()

	ss.statsFile, err = os.Create(ss.config.LogFilePath + "/edssser_stats.txt")
	if err != nil {
		return err
	}

	_, err = ss.statsFile.Write([]byte(stats.String()))
	if err != nil {
		return err
	}

	return ss.statsFile.Close()
}

type Stats struct {
	TotalWritten                         int
	TotalTime, MinTime, MaxTime, AvgTime time.Duration
	// Deviation ?
}

func (stats Stats) Finalize() Stats {
	if stats.TotalTime != 0 {
		stats.AvgTime = stats.TotalTime / time.Duration(stats.TotalWritten)
	}
	return stats
}

func (stats Stats) String() string {
	return fmt.Sprintf(`
TotalWritten %d
TotalWritingTime %v
MaxTime %s
MinTime %s
AvgTime %s
`,
		stats.TotalWritten,
		stats.TotalTime,
		stats.MaxTime,
		stats.MinTime,
		stats.AvgTime,
	)
}
