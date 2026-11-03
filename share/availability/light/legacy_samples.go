package light

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// legacySamplesPrefix is where the removed bitswap client stored fetched samples.
var legacySamplesPrefix = datastore.NewKey("blocks")

// RemoveLegacySamples deletes samples left in the datastore by the removed bitswap client,
// which nothing prunes anymore. It is a no-op once they are gone.
func RemoveLegacySamples(ctx context.Context, ds datastore.Batching) error {
	res, err := ds.Query(ctx, query.Query{Prefix: legacySamplesPrefix.String(), KeysOnly: true})
	if err != nil {
		return fmt.Errorf("query legacy samples: %w", err)
	}
	defer res.Close()

	batch, err := ds.Batch(ctx)
	if err != nil {
		return fmt.Errorf("new batch: %w", err)
	}

	var removed int
	for {
		r, ok := res.NextSync()
		if !ok {
			break
		}
		if r.Error != nil {
			return fmt.Errorf("iterate legacy samples: %w", r.Error)
		}
		if err := batch.Delete(ctx, datastore.NewKey(r.Key)); err != nil {
			return fmt.Errorf("delete legacy sample: %w", err)
		}
		removed++
		if removed%writeBatchSize == 0 {
			if err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("commit legacy samples removal: %w", err)
			}
			if batch, err = ds.Batch(ctx); err != nil {
				return fmt.Errorf("new batch: %w", err)
			}
		}
	}
	if err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("commit legacy samples removal: %w", err)
	}

	if removed > 0 {
		log.Infow("removed samples left by the bitswap client", "count", removed)
	}
	return nil
}
