package block

import (
	"context"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/tendermint/tendermint/pkg/da"
)

func (s *Service) StoreBlockData(ctx context.Context, data *ExtendedBlockData) error {
	shares := ipld.ExtractODSShares(data)
	// TODO @renaynay: it's inefficient that we generate the EDS twice: once in event loop
	// once in the IPLD plugin.
	_, err := ipld.PutData(ctx, shares, s.store)
	return err
}

func (s *Service) GetBlockData(ctx context.Context, dah *da.DataAvailabilityHeader) (*ExtendedBlockData, error) {
	return ipld.RetrieveData(ctx, dah, s.store, rsmt2d.NewRSGF8Codec())
}
