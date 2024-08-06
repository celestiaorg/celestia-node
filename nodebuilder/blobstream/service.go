package blobstream

import (
	"context"

	logging "github.com/ipfs/go-log/v2"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
)

var _ Module = (*Service)(nil)

var log = logging.Logger("go-blobstream")

type Service struct {
	headerServ headerServ.Module
}

func NewService(headerMod headerServ.Module) *Service {
	return &Service{
		headerServ: headerMod,
	}
}

// GetDataRootTupleRoot collects the data roots over a provided ordered range of blocks,
// and then creates a new Merkle root of those data roots. The range is end exclusive.
func (s *Service) GetDataRootTupleRoot(ctx context.Context, start, end uint64) (*DataRootTupleRoot, error) {
	log.Debugw("validating the data commitment range", "start", start, "end", end)
	err := s.validateDataRootTupleRootRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)
	encodedDataRootTuples, err := s.fetchEncodedDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("hashing the data root tuples", "start", start, "end", end)
	root, err := hashDataRootTuples(encodedDataRootTuples)
	if err != nil {
		return nil, err
	}
	// Create data commitment
	dataRootTupleRoot := DataRootTupleRoot(root)
	return &dataRootTupleRoot, nil
}

// GetDataRootTupleInclusionProof creates an inclusion proof for the data root of block
// height `height` in the set of blocks defined by `start` and `end`. The range
// is end exclusive.
func (s *Service) GetDataRootTupleInclusionProof(
	ctx context.Context,
	height, start, end uint64,
) (*DataRootTupleInclusionProof, error) {
	log.Debugw(
		"validating the data root inclusion proof request",
		"start",
		start,
		"end",
		end,
		"height",
		height,
	)
	err := s.validateDataRootInclusionProofRequest(ctx, height, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)

	encodedDataRootTuples, err := s.fetchEncodedDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("proving the data root tuples", "start", start, "end", end)
	proof, err := proveDataRootTuples(encodedDataRootTuples, start, height)
	if err != nil {
		return nil, err
	}
	dataRootTupleInclusionProof := DataRootTupleInclusionProof(proof)
	return &dataRootTupleInclusionProof, nil
}
