package blobstream

import (
	bytes2 "bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"

	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/crypto/merkle"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	pkgproof "github.com/celestiaorg/celestia-app/pkg/proof"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/blob"
	nodeblob "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	shareServ "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/share"
)

var _ Module = (*Service)(nil)

var log = logging.Logger("go-blobstream")

type Service struct {
	blobServ   nodeblob.Module
	headerServ headerServ.Module
	shareServ  shareServ.Module
}

func NewService(blobMod nodeblob.Module, headerMod headerServ.Module, shareMod shareServ.Module) *Service {
	return &Service{
		blobServ:   blobMod,
		headerServ: headerMod,
		shareServ:  shareMod,
	}
}

// GetDataCommitment collects the data roots over a provided ordered range of blocks,
// and then creates a new Merkle root of those data roots. The range is end exclusive.
func (s *Service) GetDataCommitment(ctx context.Context, start, end uint64) (*ResultDataCommitment, error) {
	log.Debugw("validating the data commitment range", "start", start, "end", end)
	err := s.validateDataCommitmentRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)
	tuples, err := s.fetchDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("hashing the data root tuples", "start", start, "end", end)
	root, err := hashDataRootTuples(tuples)
	if err != nil {
		return nil, err
	}
	// Create data commitment
	return &ResultDataCommitment{DataCommitment: root}, nil
}

// GetDataRootInclusionProof creates an inclusion proof for the data root of block
// height `height` in the set of blocks defined by `start` and `end`. The range
// is end exclusive.
func (s *Service) GetDataRootInclusionProof(
	ctx context.Context,
	height int64,
	start,
	end uint64,
) (*ResultDataRootInclusionProof, error) {
	log.Debugw(
		"validating the data root inclusion proof request",
		"start",
		start,
		"end",
		end,
		"height",
		height,
	)
	err := s.validateDataRootInclusionProofRequest(ctx, uint64(height), start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)
	tuples, err := s.fetchDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("proving the data root tuples", "start", start, "end", end)
	proof, err := proveDataRootTuples(tuples, height)
	if err != nil {
		return nil, err
	}
	return &ResultDataRootInclusionProof{Proof: *proof}, nil
}

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

	currentHeader, err := s.headerServ.NetworkHead(ctx)
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

	currentLocalHeader, err := s.headerServ.LocalHead(ctx)
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
		block, err := s.headerServ.GetByHeight(ctx, height)
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

// ProveShares generates a share proof for a share range.
// Note: queries the whole EDS to generate the proof.
// This can be improved by selecting the set of shares that will need to be used to create
// the proof and only querying them. However, that would require re-implementing the logic
// in Core. Also, core also queries the whole EDS to generate the proof. So, it's fine for
// now. In the future, when blocks get way bigger, we should revisit this and improve it.
func (s *Service) ProveShares(ctx context.Context, height, start, end uint64) (*ResultShareProof, error) {
	log.Debugw("proving share range", "start", start, "end", end, "height", height)
	if height == 0 {
		return nil, fmt.Errorf("height cannot be equal to 0")
	}
	if start == end {
		return nil, fmt.Errorf("start share cannot be equal to end share")
	}
	if start > end {
		return nil, fmt.Errorf("start share %d cannot be greater than end share %d", start, end)
	}

	log.Debugw("getting extended header", "height", height)
	extendedHeader, err := s.headerServ.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	log.Debugw("getting eds", "height", height)
	eds, err := s.shareServ.GetEDS(ctx, extendedHeader)
	if err != nil {
		return nil, err
	}

	startInt, err := uint64ToInt(start)
	if err != nil {
		return nil, err
	}
	endInt, err := uint64ToInt(end)
	if err != nil {
		return nil, err
	}
	odsShares, err := shares.FromBytes(eds.FlattenedODS())
	if err != nil {
		return nil, err
	}
	nID, err := pkgproof.ParseNamespace(odsShares, startInt, endInt)
	if err != nil {
		return nil, err
	}
	log.Debugw("generating the share proof", "start", start, "end", end, "height", height)
	proof, err := pkgproof.NewShareInclusionProofFromEDS(eds, nID, shares.NewRange(startInt, endInt))
	if err != nil {
		return nil, err
	}
	return &ResultShareProof{ShareProof: proof}, nil
}

// ProveCommitment generates a commitment proof for a share commitment.
// It takes as argument the height of the block containing the blob of data, its
// namespace and its share commitment.
// Note: queries the whole EDS to generate the proof.
// This can be improved once `GetProof` returns the proof only for the blob and not the whole
// namespace.
func (s *Service) ProveCommitment(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	shareCommitment []byte,
) (*ResultCommitmentProof, error) {
	log.Debugw("proving share commitment", "height", height, "commitment", shareCommitment, "namespace", namespace)
	if height == 0 {
		return nil, fmt.Errorf("height cannot be equal to 0")
	}

	// get the blob to compute the subtree roots
	log.Debugw(
		"getting the blob",
		"height",
		height,
		"commitment",
		shareCommitment,
		"namespace",
		namespace,
	)
	blb, err := s.blobServ.Get(ctx, height, namespace, shareCommitment)
	if err != nil {
		return nil, err
	}

	log.Debugw(
		"converting the blob to shares",
		"height",
		height,
		"commitment",
		shareCommitment,
		"namespace",
		namespace,
	)
	blobShares, err := blob.BlobsToShares(blb)
	if err != nil {
		return nil, err
	}
	if len(blobShares) == 0 {
		return nil, fmt.Errorf("the blob shares for commitment %s are empty", hex.EncodeToString(shareCommitment))
	}

	// get the extended header
	log.Debugw(
		"getting the extended header",
		"height",
		height,
	)
	extendedHeader, err := s.headerServ.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	log.Debugw("getting eds", "height", height)
	eds, err := s.shareServ.GetEDS(ctx, extendedHeader)
	if err != nil {
		return nil, err
	}

	// find the blob shares in the EDS
	blobSharesStartIndex := -1
	for index, share := range eds.FlattenedODS() {
		if bytes2.Equal(share, blobShares[0]) {
			blobSharesStartIndex = index
		}
	}
	if blobSharesStartIndex < 0 {
		return nil, fmt.Errorf("couldn't find the blob shares in the ODS")
	}

	nID, err := appns.From(namespace)
	if err != nil {
		return nil, err
	}

	log.Debugw(
		"generating the blob share proof for commitment",
		"commitment",
		shareCommitment,
		"start_share",
		blobSharesStartIndex,
		"end_share",
		blobSharesStartIndex+len(blobShares),
		"height",
		height,
	)
	sharesProof, err := pkgproof.NewShareInclusionProofFromEDS(
		eds,
		nID,
		shares.NewRange(blobSharesStartIndex, blobSharesStartIndex+len(blobShares)),
	)
	if err != nil {
		return nil, err
	}

	// convert the shares to row root proofs to nmt proofs
	nmtProofs := make([]*nmt.Proof, 0)
	for _, proof := range sharesProof.ShareProofs {
		nmtProof := nmt.NewInclusionProof(int(proof.Start),
			int(proof.End),
			proof.Nodes,
			true)
		nmtProofs = append(
			nmtProofs,
			&nmtProof,
		)
	}

	// compute the subtree roots of the blob shares
	log.Debugw(
		"computing the subtree roots",
		"height",
		height,
		"commitment",
		shareCommitment,
		"namespace",
		namespace,
	)
	subtreeRoots := make([][]byte, 0)
	dataCursor := 0
	for _, proof := range nmtProofs {
		// TODO: do we want directly use the default subtree root threshold
		// or want to allow specifying which version to use?
		ranges, err := nmt.ToLeafRanges(
			proof.Start(),
			proof.End(),
			shares.SubTreeWidth(len(blobShares), appconsts.DefaultSubtreeRootThreshold),
		)
		if err != nil {
			return nil, err
		}
		roots, err := computeSubtreeRoots(
			blobShares[dataCursor:dataCursor+proof.End()-proof.Start()],
			ranges,
			proof.Start(),
		)
		if err != nil {
			return nil, err
		}
		subtreeRoots = append(subtreeRoots, roots...)
		dataCursor += proof.End() - proof.Start()
	}

	log.Debugw(
		"successfully proved the share commitment",
		"height",
		height,
		"commitment",
		shareCommitment,
		"namespace",
		namespace,
	)
	commitmentProof := CommitmentProof{
		SubtreeRoots:      subtreeRoots,
		SubtreeRootProofs: nmtProofs,
		NamespaceID:       namespace.ID(),
		RowProof:          sharesProof.RowProof,
		NamespaceVersion:  namespace.Version(),
	}

	return &ResultCommitmentProof{CommitmentProof: commitmentProof}, nil
}

// computeSubtreeRoots takes a set of shares and ranges and returns the corresponding subtree roots.
// the offset is the number of shares that are before the subtree roots we're calculating.
func computeSubtreeRoots(shares []share.Share, ranges []nmt.LeafRange, offset int) ([][]byte, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("cannot compute subtree roots for an empty shares list")
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("cannot compute subtree roots for an empty ranges list")
	}
	if offset < 0 {
		return nil, fmt.Errorf("the offset %d cannot be stricly negative", offset)
	}

	// create a tree containing the shares to generate their subtree roots
	tree := nmt.New(
		appconsts.NewBaseHashFunc(),
		nmt.IgnoreMaxNamespace(true),
		nmt.NamespaceIDSize(share.NamespaceSize),
	)
	for _, sh := range shares {
		leafData := make([]byte, 0)
		leafData = append(append(leafData, share.GetNamespace(sh)...), sh...)
		err := tree.Push(leafData)
		if err != nil {
			return nil, err
		}
	}

	// generate the subtree roots
	subtreeRoots := make([][]byte, 0)
	for _, rg := range ranges {
		root, err := tree.ComputeSubtreeRoot(rg.Start-offset, rg.End-offset)
		if err != nil {
			return nil, err
		}
		subtreeRoots = append(subtreeRoots, root)
	}
	return subtreeRoots, nil
}

func uint64ToInt(number uint64) (int, error) {
	if number >= math.MaxInt {
		return 0, fmt.Errorf("number %d is higher than max int %d", number, math.MaxInt)
	}
	return int(number), nil
}

// ProveSubtreeRootToCommitment generates a subtree root to share commitment inclusion proof.
// Note: this method is not part of the API. It will not be served by any endpoint, however,
// it can be called directly programmatically.
func ProveSubtreeRootToCommitment(
	subtreeRoots [][]byte,
	subtreeRootIndex uint64,
) (*ResultSubtreeRootToCommitmentProof, error) {
	_, proofs := merkle.ProofsFromByteSlices(subtreeRoots)
	return &ResultSubtreeRootToCommitmentProof{
		SubtreeRootToCommitmentProof: SubtreeRootToCommitmentProof{
			Proof: *proofs[subtreeRootIndex],
		},
	}, nil
}

// ProveShareToSubtreeRoot generates a share to subtree root inclusion proof
// Note: this method is not part of the API. It will not be served by any endpoint, however,
// it can be called directly programmatically.
func ProveShareToSubtreeRoot(
	shares [][]byte,
	shareIndex uint64,
) (*ResultShareToSubtreeRootProof, error) {
	_, proofs := merkle.ProofsFromByteSlices(shares)
	return &ResultShareToSubtreeRootProof{
		ShareToSubtreeRootProof: ShareToSubtreeRootProof{
			Proof: *proofs[shareIndex],
		},
	}, nil
}
