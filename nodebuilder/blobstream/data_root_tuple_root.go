package blobstream

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/bytes"

	nodeheader "github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
)

// DataRootTupleRoot is the root of the merkle tree created
// from a set of data root tuples.
type DataRootTupleRoot bytes.HexBytes

// DataRootTupleInclusionProof is the binary merkle
// inclusion proof of a height to a data commitment.
type DataRootTupleInclusionProof *merkle.Proof

// padBytes Pad bytes to given length
func padBytes(byt []byte, length int) ([]byte, error) {
	l := len(byt)
	if l > length {
		return nil, fmt.Errorf(
			"cannot pad bytes because length of bytes array: %d is greater than given length: %d",
			l,
			length,
		)
	}
	if l == length {
		return byt, nil
	}
	tmp := make([]byte, length)
	copy(tmp[length-l:], byt)
	return tmp, nil
}

// to32PaddedHexBytes takes a number and returns its hex representation padded to 32 bytes.
// Used to mimic the result of `abi.encode(number)` in Ethereum.
func to32PaddedHexBytes(number uint64) ([]byte, error) {
	hexRepresentation := strconv.FormatUint(number, 16)
	// Make sure hex representation has even length.
	// The `strconv.FormatUint` can return odd length hex encodings.
	// For example, `strconv.FormatUint(10, 16)` returns `a`.
	// Thus, we need to pad it.
	if len(hexRepresentation)%2 == 1 {
		hexRepresentation = "0" + hexRepresentation
	}
	hexBytes, hexErr := hex.DecodeString(hexRepresentation)
	if hexErr != nil {
		return nil, hexErr
	}
	paddedBytes, padErr := padBytes(hexBytes, 32)
	if padErr != nil {
		return nil, padErr
	}
	return paddedBytes, nil
}

// encodeDataRootTuple takes a height and a data root, and returns the equivalent of
// `abi.encode(...)` in Ethereum.
// The encoded type is a dataRootTuple, which has the following ABI:
//
//	{
//	  "components":[
//	     {
//	        "internalType":"uint256",
//	        "name":"height",
//	        "type":"uint256"
//	     },
//	     {
//	        "internalType":"bytes32",
//	        "name":"dataRoot",
//	        "type":"bytes32"
//	     },
//	     {
//	        "internalType":"structDataRootTuple",
//	        "name":"_tuple",
//	        "type":"tuple"
//	     }
//	  ]
//	}
//
// padding the hex representation of the height padded to 32 bytes concatenated to the data root.
// For more information, refer to:
// https://github.com/celestiaorg/blobstream-contracts/blob/master/src/DataRootTuple.sol
func encodeDataRootTuple(height uint64, dataRoot [32]byte) ([]byte, error) {
	paddedHeight, err := to32PaddedHexBytes(height)
	if err != nil {
		return nil, err
	}
	return append(paddedHeight, dataRoot[:]...), nil
}

// dataRootTupleRootBlocksLimit The maximum number of blocks to be used to create a data commitment.
// It's a local parameter to protect the API from creating unnecessarily large commitments.
const dataRootTupleRootBlocksLimit = 10_000 // ~27 hours of blocks assuming 10-second blocks.

// validateDataRootTupleRootRange runs basic checks on the ascending sorted list of
// heights that will be used subsequently in generating data commitments over
// the defined set of heights by ensuring the range exists in the chain.
func (s *Service) validateDataRootTupleRootRange(ctx context.Context, start, end uint64) error {
	if start == 0 {
		return header.ErrHeightZero
	}
	if start >= end {
		return fmt.Errorf("end block is smaller or equal to the start block")
	}

	heightsRange := end - start
	if heightsRange > uint64(dataRootTupleRootBlocksLimit) {
		return fmt.Errorf("the query exceeds the limit of allowed blocks %d", dataRootTupleRootBlocksLimit)
	}

	currentLocalHeader, err := s.headerServ.LocalHead(ctx)
	if err != nil {
		return fmt.Errorf("could not get the local head to validate the data root tuple root range: %w", err)
	}
	// the data commitment range is end exclusive
	if end > currentLocalHeader.Height()+1 {
		return fmt.Errorf(
			"end block %d is higher than local chain height %d. Wait for the node until it syncs up to %d",
			end,
			currentLocalHeader.Height(),
			end,
		)
	}
	return nil
}

// hashDataRootTuples hashes a list of encoded blocks data root tuples, i.e., height, data root and
// square size, then returns their merkle root.
func hashDataRootTuples(encodedDataRootTuples [][]byte) ([]byte, error) {
	if len(encodedDataRootTuples) == 0 {
		return nil, fmt.Errorf("cannot hash an empty list of encoded data root tuples")
	}
	root := merkle.HashFromByteSlices(encodedDataRootTuples)
	return root, nil
}

// validateDataRootInclusionProofRequest validates the request to generate a data root
// inclusion proof.
func (s *Service) validateDataRootInclusionProofRequest(
	ctx context.Context,
	height, start, end uint64,
) error {
	err := s.validateDataRootTupleRootRange(ctx, start, end)
	if err != nil {
		return err
	}
	if height < start || height >= end {
		return fmt.Errorf(
			"height %d should be in the end exclusive interval first_block %d last_block %d",
			height,
			start,
			end,
		)
	}
	return nil
}

// proveDataRootTuples returns the merkle inclusion proof for a height.
// expects the list of encoded data root tuples to be ordered and the heights to be consecutive.
func proveDataRootTuples(encodedDataRootTuples [][]byte, rangeStartHeight, height uint64) (*merkle.Proof, error) {
	if len(encodedDataRootTuples) == 0 {
		return nil, fmt.Errorf("cannot prove an empty list of encoded data root tuples")
	}
	if height == 0 || rangeStartHeight == 0 {
		return nil, header.ErrHeightZero
	}
	_, proofs := merkle.ProofsFromByteSlices(encodedDataRootTuples)
	return proofs[height-rangeStartHeight], nil
}

// fetchEncodedDataRootTuples takes an end exclusive range of heights and fetches its
// corresponding data root tuples.
// end is not included in the range.
func (s *Service) fetchEncodedDataRootTuples(ctx context.Context, start, end uint64) ([][]byte, error) {
	encodedDataRootTuples := make([][]byte, 0, end-start)
	headers := make([]*nodeheader.ExtendedHeader, 0, end-start)

	startHeader, err := s.headerServ.GetByHeight(ctx, start)
	if err != nil {
		return nil, err
	}
	headers = append(headers, startHeader)

	headerRange, err := s.headerServ.GetRangeByHeight(ctx, startHeader, end)
	if err != nil {
		return nil, err
	}
	headers = append(headers, headerRange...)

	for _, header := range headers {
		encodedDataRootTuple, err := encodeDataRootTuple(header.Height(), *(*[32]byte)(header.DataHash))
		if err != nil {
			return nil, err
		}
		encodedDataRootTuples = append(encodedDataRootTuples, encodedDataRootTuple)
	}
	return encodedDataRootTuples, nil
}
