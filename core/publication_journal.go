package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	dssync "github.com/ipfs/go-datastore/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/store"
)

const publicationRetryBatchSize = 32

var publicationJournalPrefix = datastore.NewKey("core").ChildString("pending_publications")

type publicationJournal struct {
	datastore.Datastore
}

func newPublicationJournal(ds datastore.Datastore) *publicationJournal {
	return &publicationJournal{
		Datastore: dssync.MutexWrap(namespace.Wrap(ds, publicationJournalPrefix)),
	}
}

func (j *publicationJournal) put(ctx context.Context, eh *header.ExtendedHeader) error {
	headerData, err := eh.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling header: %w", err)
	}

	key := publicationKey(eh.Height())
	if err := j.Put(ctx, key, headerData); err != nil {
		return fmt.Errorf("putting publication: %w", err)
	}
	if err := j.Sync(ctx, key); err != nil {
		return fmt.Errorf("syncing publication: %w", err)
	}
	return nil
}

func (j *publicationJournal) pending(
	ctx context.Context,
	after string,
	through string,
) ([]*header.ExtendedHeader, string, error) {
	q := query.Query{
		Orders:   []query.Order{query.OrderByKey{}},
		Limit:    publicationRetryBatchSize,
		KeysOnly: true,
	}
	if after != "" {
		q.Filters = append(q.Filters, query.FilterKeyCompare{Op: query.GreaterThan, Key: after})
	}
	if through != "" {
		q.Filters = append(q.Filters, query.FilterKeyCompare{Op: query.LessThanOrEqual, Key: through})
	}
	results, err := j.Query(ctx, q)
	if err != nil {
		return nil, "", fmt.Errorf("querying publications: %w", err)
	}
	defer results.Close()

	entries, err := results.Rest()
	if err != nil {
		return nil, "", fmt.Errorf("reading publications: %w", err)
	}
	publications := make([]*header.ExtendedHeader, 0, len(entries))
	for _, entry := range entries {
		key := datastore.RawKey(entry.Key)
		value, err := j.Get(ctx, key)
		if errors.Is(err, datastore.ErrNotFound) {
			continue
		}
		if err != nil {
			log.Errorw("listener: getting pending publication", "key", entry.Key, "err", err)
			continue
		}

		eh, err := decodePendingPublication(entry.Key, value)
		if err != nil {
			log.Errorw("listener: decoding pending publication", "key", entry.Key, "err", err)
			if err := j.Delete(ctx, key); err != nil {
				log.Errorw("listener: removing invalid pending publication", "key", entry.Key, "err", err)
			}
			continue
		}
		publications = append(publications, eh)
	}
	if len(entries) == 0 {
		return publications, "", nil
	}
	return publications, entries[len(entries)-1].Key, nil
}

func (j *publicationJournal) lastKey(ctx context.Context) (string, error) {
	results, err := j.Query(ctx, query.Query{
		Orders:   []query.Order{query.OrderByKeyDescending{}},
		Limit:    1,
		KeysOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("querying last publication: %w", err)
	}
	defer results.Close()

	entries, err := results.Rest()
	if err != nil {
		return "", fmt.Errorf("reading last publication: %w", err)
	}
	if len(entries) == 0 {
		return "", nil
	}
	return entries[0].Key, nil
}

func (j *publicationJournal) remove(ctx context.Context, height uint64) error {
	if err := j.Delete(ctx, publicationKey(height)); err != nil {
		return fmt.Errorf("deleting publication: %w", err)
	}
	return nil
}

func publicationKey(height uint64) datastore.Key {
	return datastore.NewKey(fmt.Sprintf("%020d", height))
}

func decodePendingPublication(key string, data []byte) (*header.ExtendedHeader, error) {
	eh := new(header.ExtendedHeader)
	if err := eh.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshaling header: %w", err)
	}
	if expected := publicationKey(eh.Height()).String(); key != expected {
		return nil, fmt.Errorf("height key mismatch: got %s, expected %s", key, expected)
	}
	if err := eh.Validate(); err != nil {
		return nil, fmt.Errorf("validating header: %w", err)
	}
	return eh, nil
}

func (cl *Listener) retryPendingPublications(ctx context.Context) {
	if cl.publications == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publicationRetryTimeout)
	defer cancel()

	// Keep each sweep finite so newly journaled heights cannot indefinitely
	// postpone retrying an older failed publication.
	if cl.publicationSweepEnd == "" || cl.publicationCursor >= cl.publicationSweepEnd {
		cl.publicationCursor = ""
		sweepEnd, err := cl.publications.lastKey(ctx)
		if err != nil {
			log.Errorw("listener: loading pending publication boundary", "err", err)
			return
		}
		cl.publicationSweepEnd = sweepEnd
		if cl.publicationSweepEnd == "" {
			return
		}
	}

	publications, cursor, err := cl.publications.pending(
		ctx,
		cl.publicationCursor,
		cl.publicationSweepEnd,
	)
	if err != nil {
		log.Errorw("listener: loading pending publications", "err", err)
		return
	}
	if len(publications) == 0 {
		cl.advancePublicationSweep(cursor)
		return
	}

	for _, eh := range publications {
		if ctx.Err() != nil {
			return
		}

		height := eh.Height()
		cl.publicationCursor = publicationKey(height).String()
		// Old announcements no longer help peers, even when an archival node
		// retains the underlying EDS indefinitely.
		if !availability.IsWithinWindow(eh.Time(), cl.availabilityWindow) {
			if err := cl.publications.remove(ctx, height); err != nil {
				log.Errorw("listener: removing expired publication", "height", height, "err", err)
			}
			continue
		}
		if cl.chainID != "" && eh.ChainID() != cl.chainID {
			log.Errorw("listener: pending publication is for the wrong chain", "height", height,
				"expected", cl.chainID, "received", eh.ChainID())
			if err := cl.publications.remove(ctx, height); err != nil {
				log.Errorw("listener: removing wrong-chain publication", "height", height, "err", err)
			}
			continue
		}

		accessor, err := cl.store.GetPersistedByHeight(ctx, height)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			log.Errorw("listener: opening filesystem-backed EDS for pending publication",
				"height", height, "err", err)
			continue
		}
		dataHash, err := accessor.DataHash(ctx)
		utils.CloseAndLog(log.With("height", height), "pending publication EDS", accessor)
		if err != nil {
			log.Errorw("listener: reading EDS for pending publication", "height", height, "err", err)
			continue
		}
		if !bytes.Equal(dataHash, eh.DataHash) {
			log.Errorw("listener: pending publication does not match stored EDS", "height", height)
			if err := cl.publications.remove(ctx, height); err != nil {
				log.Errorw("listener: removing mismatched publication", "height", height, "err", err)
			}
			continue
		}

		if err := cl.broadcast(ctx, eh, false); err != nil {
			continue
		}
		if err := cl.publications.remove(ctx, height); err != nil {
			log.Errorw("listener: removing pending publication", "height", height, "err", err)
		}
	}
	cl.advancePublicationSweep(cursor)
}

func (cl *Listener) advancePublicationSweep(cursor string) {
	if cursor == "" || cursor >= cl.publicationSweepEnd {
		cl.publicationCursor = ""
		cl.publicationSweepEnd = ""
		return
	}
	cl.publicationCursor = cursor
}
