package share

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/boxo/blockservice"

	"github.com/celestiaorg/celestia-app/pkg/da"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func newShareModule(getter share.Getter, avail share.Availability, header headerServ.Module) Module {
	return &module{getter, avail, header}
}

// ensureEmptyCARExists adds an empty EDS to the provided EDS store.
func ensureEmptyCARExists(ctx context.Context, store *eds.Store) error {
	emptyEDS := share.EmptyExtendedDataSquare()
	emptyDAH, err := da.NewDataAvailabilityHeader(emptyEDS)
	if err != nil {
		return err
	}

	err = store.Put(ctx, emptyDAH.Hash(), emptyEDS)
	if errors.Is(err, dagstore.ErrShardExists) {
		return nil
	}
	return err
}

// ensureEmptyEDSInBS checks if the given DAG contains an empty block data square.
// If it does not, it stores an empty block. This optimization exists to prevent
// redundant storing of empty block data so that it is only stored once and returned
// upon request for a block with an empty data square.
func ensureEmptyEDSInBS(ctx context.Context, bServ blockservice.BlockService) error {
	_, err := ipld.AddShares(ctx, share.EmptyBlockShares(), bServ)
	return err
}

func lightGetter(
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, ipldGetter)
	return getters.NewCascadeGetter(cascade)
}

// ShrexGetter is added to bridge nodes for the case that a shard is removed
// after detected shard corruption. This ensures the block is fetched and stored
// by shrex the next time the data is retrieved (meaning shard recovery is
// manual after corruption is detected).
func bridgeGetter(
	storeGetter *getters.StoreGetter,
	shrexGetter *getters.ShrexGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	return getters.NewCascadeGetter(cascade)
}

func fullGetter(
	storeGetter *getters.StoreGetter,
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, ipldGetter)
	return getters.NewCascadeGetter(cascade)
}
