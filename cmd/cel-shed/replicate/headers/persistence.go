package headers

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/header"
)

// PersistHeadKey writes go-header's /headers/head pointer. We require the
// matching height entry to already be on disk: never publish a head pointer
// that points past the latest durably flushed header. Exported so that
// replicate.Run can call it after stopping the shared hstore.
func PersistHeadKey(ds datastore.Datastore, h *header.ExtendedHeader) error {
	if h == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	heightKey := datastore.NewKey(fmt.Sprintf("/headers/%d", h.Height()))
	onDisk, err := ds.Get(ctx, heightKey)
	if err != nil {
		return fmt.Errorf("confirm committed header height %d: %w", h.Height(), err)
	}
	if !bytes.Equal(onDisk, []byte(h.Hash())) {
		return fmt.Errorf("confirm committed header height %d: on-disk hash %X != appended %s",
			h.Height(), onDisk, h.Hash())
	}

	jsonHash, err := h.Hash().MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal head hash: %w", err)
	}
	if err := ds.Put(ctx, datastore.NewKey("/headers/head"), jsonHash); err != nil {
		return fmt.Errorf("write /headers/head: %w", err)
	}
	return nil
}
