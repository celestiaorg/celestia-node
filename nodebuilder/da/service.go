package da

import (
	"context"
	"encoding/binary"
	"log"
	"math"
	"strings"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/rollkit/go-da"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

// heightLen is a length (in bytes) of serialized height.
//
// This is 8 as uint64 consist of 8 bytes.
const heightLen = 8

type Service struct {
	blobServ  blob.Service
	namespace share.Namespace
}

// MaxBlobSize returns the max blob size
func (s *Service) MaxBlobSize(context.Context) (uint64, error) {
	// TODO: pass-through query to node, app
	return appconsts.DefaultMaxBytes, nil
}

// Get returns Blob for each given ID, or an error.
func (s *Service) Get(ctx context.Context, ids []da.ID) ([]da.Blob, error) {
	blobs := make([]da.Blob, 0, len(ids))
	for _, id := range ids {
		height, commitment := splitID(id)
		blob, err := s.blobServ.Get(ctx, height, s.namespace, commitment)
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob.Data)
	}
	return blobs, nil
}

// GetIDs returns IDs of all Blobs located in DA at given height.
func (s *Service) GetIDs(ctx context.Context, height uint64) ([]da.ID, error) {
	var ids []da.ID
	blobs, err := s.blobServ.GetAll(ctx, height, []share.Namespace{s.namespace})
	if err != nil {
		if strings.Contains(err.Error(), blob.ErrBlobNotFound.Error()) {
			return nil, nil
		}
		return nil, err
	}
	for _, b := range blobs {
		ids = append(ids, makeID(height, b.Commitment))
	}
	return ids, nil
}

// Commit creates a Commitment for each given Blob.
func (s *Service) Commit(_ context.Context, daBlobs []da.Blob) ([]da.Commitment, error) {
	_, commitments, err := s.blobsAndCommitments(daBlobs)
	return commitments, err
}

// Submit submits the Blobs to Data Availability layer.
func (s *Service) Submit(ctx context.Context, daBlobs []da.Blob, opts *da.SubmitOptions) ([]da.ID, []da.Proof, error) {
	blobs, commitments, err := s.blobsAndCommitments(daBlobs)
	if err != nil {
		return nil, nil, err
	}
	options := blob.DefaultSubmitOptions()
	if opts.GasPrice >= 0 {
		blobSizes := make([]uint32, len(blobs))
		for i, blob := range blobs {
			blobSizes[i] = uint32(len(blob.Data))
		}
		options.GasLimit = types.EstimateGas(blobSizes, appconsts.DefaultGasPerBlobByte, auth.DefaultTxSizeCostPerByte)
		options.Fee = sdktypes.NewInt(int64(math.Ceil(opts.GasPrice * float64(options.GasLimit)))).Int64()
	}
	height, err := s.blobServ.Submit(ctx, blobs, options)
	if err != nil {
		return nil, nil, err
	}
	log.Println("successfully submitted blobs", "height", height, "gas", options.GasLimit, "fee", options.Fee)
	ids := make([]da.ID, len(daBlobs))
	proofs := make([]da.Proof, len(daBlobs))
	for i, commitment := range commitments {
		ids[i] = makeID(height, commitment)
		proof, err := s.blobServ.GetProof(ctx, height, s.namespace, commitment)
		if err != nil {
			return nil, nil, err
		}
		// TODO(tzdybal): does always len(*proof) == 1?
		proofs[i], err = (*proof)[0].MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
	}
	return ids, proofs, nil
}

// blobsAndCommitments converts []da.Blob to []*blob.Blob and generates corresponding []da.Commitment
func (s *Service) blobsAndCommitments(daBlobs []da.Blob) ([]*blob.Blob, []da.Commitment, error) {
	blobs := make([]*blob.Blob, 0, len(daBlobs))
	commitments := make([]da.Commitment, 0, len(daBlobs))
	for _, daBlob := range daBlobs {
		b, err := blob.NewBlobV0(s.namespace, daBlob)
		if err != nil {
			return nil, nil, err
		}
		blobs = append(blobs, b)

		commitment, err := types.CreateCommitment(&b.Blob)
		if err != nil {
			return nil, nil, err
		}
		commitments = append(commitments, commitment)
	}
	return blobs, commitments, nil
}

// Validate validates Commitments against the corresponding Proofs. This should be possible without
// retrieving the Blobs.
func (s *Service) Validate(ctx context.Context, ids []da.ID, daProofs []da.Proof) ([]bool, error) {
	included := make([]bool, len(ids))
	proofs := make([]*blob.Proof, len(ids))
	for _, daProof := range daProofs {
		nmtProof := &nmt.Proof{}
		if err := nmtProof.UnmarshalJSON(daProof); err != nil {
			return nil, err
		}
		proof := &blob.Proof{nmtProof}
		proofs = append(proofs, proof)
	}
	for i, id := range ids {
		height, commitment := splitID(id)
		// TODO(tzdybal): for some reason, if proof doesn't match commitment, API returns (false, "blob: invalid proof")
		//    but analysis of the code in celestia-node implies this should never happen - maybe it's caused by openrpc?
		//    there is no way of gently handling errors here, but returned value is fine for us
		isIncluded, _ := s.blobServ.Included(ctx, height, s.namespace, proofs[i], commitment)
		included = append(included, isIncluded)
	}
	return included, nil
}

func makeID(height uint64, commitment da.Commitment) da.ID {
	id := make([]byte, heightLen+len(commitment))
	binary.LittleEndian.PutUint64(id, height)
	copy(id[heightLen:], commitment)
	return id
}

func splitID(id da.ID) (uint64, da.Commitment) {
	if len(id) <= heightLen {
		return 0, nil
	}
	return binary.LittleEndian.Uint64(id[:heightLen]), id[heightLen:]
}
