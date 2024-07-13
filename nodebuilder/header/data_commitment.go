package header

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/bytes"
)

// DataCommitment is the data root tuple root.
type DataCommitment bytes.HexBytes

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

// To32PaddedHexBytes takes a number and returns its hex representation padded to 32 bytes.
// Used to mimic the result of `abi.encode(number)` in Ethereum.
func To32PaddedHexBytes(number uint64) ([]byte, error) {
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

// DataRootTuple contains the data that will be used to create the QGB commitments.
// The commitments will be signed by orchestrators and submitted to an EVM chain via a relayer.
// For more information:
// https://github.com/celestiaorg/quantum-gravity-bridge/blob/master/src/DataRootTuple.sol
type DataRootTuple struct {
	height   uint64
	dataRoot [32]byte
}

// EncodeDataRootTuple takes a height and a data root, and returns the equivalent of
// `abi.encode(...)` in Ethereum.
// The encoded type is a DataRootTuple, which has the following ABI:
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
func EncodeDataRootTuple(height uint64, dataRoot [32]byte) ([]byte, error) {
	paddedHeight, err := To32PaddedHexBytes(height)
	if err != nil {
		return nil, err
	}
	return append(paddedHeight, dataRoot[:]...), nil
}

// dataCommitmentBlocksLimit The maximum number of blocks to be used to create a data commitment.
// It's a local parameter to protect the API from creating unnecessarily large commitments.
const dataCommitmentBlocksLimit = 10_000 // ~33 hours of blocks assuming 12-second blocks.

// validateDataCommitmentRange runs basic checks on the asc sorted list of
// heights that will be used subsequently in generating data commitments over
// the defined set of heights.
func (s *Service) validateDataCommitmentRange(ctx context.Context, start, end uint64) error {
	if start == 0 {
		return fmt.Errorf("the start block is 0")
	}
	if start >= end {
		return fmt.Errorf("end block is smaller or equal to the start block")
	}
	heightsRange := end - start
	if heightsRange > uint64(dataCommitmentBlocksLimit) {
		return fmt.Errorf("the query exceeds the limit of allowed blocks %d", dataCommitmentBlocksLimit)
	}

	currentHeader, err := s.NetworkHead(ctx)
	if err != nil {
		return err
	}
	// the data commitment range is end exclusive
	if end > currentHeader.Height()+1 {
		return fmt.Errorf(
			"end block %d is higher than current chain height %d",
			end,
			currentHeader.Height(),
		)
	}

	currentLocalHeader, err := s.LocalHead(ctx)
	if err != nil {
		return err
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

// hashDataRootTuples hashes a list of blocks data root tuples, i.e., height, data root and square
// size, then returns their merkle root.
func hashDataRootTuples(tuples []DataRootTuple) ([]byte, error) {
	if len(tuples) == 0 {
		return nil, fmt.Errorf("cannot hash an empty list of data root tuples")
	}
	dataRootEncodedTuples := make([][]byte, 0, len(tuples))
	for _, tuple := range tuples {
		encodedTuple, err := EncodeDataRootTuple(
			tuple.height,
			tuple.dataRoot,
		)
		if err != nil {
			return nil, err
		}
		dataRootEncodedTuples = append(dataRootEncodedTuples, encodedTuple)
	}
	root := merkle.HashFromByteSlices(dataRootEncodedTuples)
	return root, nil
}

// validateDataRootInclusionProofRequest validates the request to generate a data root
// inclusion proof.
func (s *Service) validateDataRootInclusionProofRequest(
	ctx context.Context,
	height, start, end uint64,
) error {
	err := s.validateDataCommitmentRange(ctx, start, end)
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
func proveDataRootTuples(tuples []DataRootTuple, height int64) (*merkle.Proof, error) {
	if len(tuples) == 0 {
		return nil, fmt.Errorf("cannot prove an empty list of tuples")
	}
	if height < 0 {
		return nil, fmt.Errorf("cannot prove a strictly negative height %d", height)
	}
	currentHeight := tuples[0].height - 1
	for _, tuple := range tuples {
		if tuple.height != currentHeight+1 {
			return nil, fmt.Errorf("the provided tuples are not consecutive %d vs %d", currentHeight, tuple.height)
		}
		currentHeight++
	}
	dataRootEncodedTuples := make([][]byte, 0, len(tuples))
	for _, tuple := range tuples {
		encodedTuple, err := EncodeDataRootTuple(
			tuple.height,
			tuple.dataRoot,
		)
		if err != nil {
			return nil, err
		}
		dataRootEncodedTuples = append(dataRootEncodedTuples, encodedTuple)
	}
	_, proofs := merkle.ProofsFromByteSlices(dataRootEncodedTuples)
	return proofs[height-int64(tuples[0].height)], nil
}

// fetchDataRootTuples takes an end exclusive range of heights and fetches its
// corresponding data root tuples.
func (s *Service) fetchDataRootTuples(ctx context.Context, start, end uint64) ([]DataRootTuple, error) {
	tuples := make([]DataRootTuple, 0, end-start)
	for height := start; height < end; height++ {
		block, err := s.GetByHeight(ctx, height)
		if err != nil {
			return nil, err
		}
		if block == nil {
			return nil, fmt.Errorf("couldn't load block %d", height)
		}
		tuples = append(tuples, DataRootTuple{
			height:   block.Height(),
			dataRoot: *(*[32]byte)(block.DataHash),
		})
	}
	return tuples, nil
}
