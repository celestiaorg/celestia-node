// Package edssser provides stress testing functionality for EDS (Extended Data Square) store
package edssser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store"
)

// Config defines parameters for EDS store stress testing
type Config struct {
	EDSSize     int
	EDSWrites   int
	WriteFrom   int
	EnableLog   bool
	LogFilePath string
	StatLogFreq int
	OpTimeout   time.Duration
}

// EDSsser performs stress testing on EDS store operations
type EDSsser struct {
	config     Config
	edsstoreMu sync.Mutex
	edsstore   *store.Store

	statsFileMu sync.Mutex
	statsFile   *os.File
}

func NewEDSsser(path string, cfg Config) (*EDSsser, error) {
	storeCfg := store.DefaultParameters()
	edsstore, err := store.NewStore(storeCfg, path)
	if err != nil {
		return nil, err
	}

	return &EDSsser{
		config:   cfg,
		edsstore: edsstore,
	}, nil
}

func (ss *EDSsser) Run(ctx context.Context) (stats Stats, err error) {
	ss.edsstoreMu.Lock()
	defer ss.edsstoreMu.Unlock()

	defer func() {
		err = errors.Join(err, ss.edsstore.Stop(ctx))
	}()

	t := &testing.T{}
	writeTo := ss.config.WriteFrom + ss.config.EDSWrites
	for height := ss.config.WriteFrom; ctx.Err() == nil && height < writeTo; height++ {
		took, err := ss.put(ctx, t, height)

		stats.TotalWritten++
		stats.TotalTime += took
		if took < stats.MinTime || stats.MinTime == 0 {
			stats.MinTime = took
		} else if took > stats.MaxTime {
			stats.MaxTime = took
		}

		if ss.config.EnableLog {
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
			if err != nil {
				fmt.Printf("ERROR put: %s, took: %v, at: %v\n", err.Error(), took, time.Now())
				continue
			}
			if took > ss.config.OpTimeout/2 {
				fmt.Println("long put", "size", ss.config.EDSSize, "took", took, "at", time.Now())
				continue
			}

			fmt.Println("square written", "size", ss.config.EDSSize, "took", took, "at", time.Now())
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

// Stats tracks performance metrics during stress testing
type Stats struct {
	TotalWritten                         int
	TotalTime, MinTime, MaxTime, AvgTime time.Duration
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

func (ss *EDSsser) put(ctx context.Context, t *testing.T, height int) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, ss.config.OpTimeout)
	if ss.config.OpTimeout == 0 {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// divide by 2 to get ODS size as expected by RandEDS
	square := edstest.RandEDS(t, ss.config.EDSSize/2)
	roots, err := share.NewAxisRoots(square)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	err = ss.edsstore.PutODSQ4(ctx, roots, uint64(height), square)
	return time.Since(now), err
}
