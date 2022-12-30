package header

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/tendermint/tendermint/types"
)

// ConstructFn aliases a function that creates a Header.
type ConstructFn = func(
	context.Context,
	*types.Block,
	*types.Commit,
	*types.ValidatorSet,
	blockservice.BlockService,
) (*ExtendedHeader, error)
