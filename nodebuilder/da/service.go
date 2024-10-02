package da

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/rollkit/go-da"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	nodeblob "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

var _ da.DA = (*Service)(nil)

var log = logging.Logger("go-da")

// heightLen is a length (in bytes) of serialized height.
//
// This is 8 as uint64 consist of 8 bytes.
const heightLen = 8

type Service struct {
	blobServ     nodeblob.Module
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error)
}

// SubmitOptions defines options for blob submission using SubmitWithOptions.
//
// SubmitWithOptions expects JSON serialized version of this struct; all fields are optional.
type SubmitOptions struct {
	// KeyName is the name of key from the keystore used to sign transactions.
	KeyName string `json:"key_name,omitempty"`

	// SignerAddress is the address that signs the transaction.
	// This address must be stored locally in the key store.
	SignerAddress string `json:"signer_address,omitempty"`

	// FeeGranterAddress specifies the account that will pay for the transaction fee.
	FeeGranterAddress string `json:"fee_granter_address,omitempty"`
}

func NewService(
	blobMod nodeblob.Module,
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error),
) *Service {
	return &Service{
		blobServ:     blobMod,
		headerGetter: headerGetter,
	}
}

// MaxBlobSize returns the max blob size
func (s *Service) MaxBlobSize(context.Context) (uint64, error) {
	return appconsts.DefaultMaxBytes, nil
}

// Get returns Blob for each given ID, or an error.
func (s *Service) Get(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Blob, error) {
	blobs := make([]da.Blob, 0, len(ids))
	for _, id := range ids {
		height, commitment := SplitID(id)
		log.Debugw("getting blob", "height", height, "commitment", commitment, "namespace", share.Namespace(ns))
		currentBlob, err := s.blobServ.Get(ctx, height, ns, commitment)
		log.Debugw("got blob", "height", height, "commitment", commitment, "namespace", share.Namespace(ns))
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, currentBlob.Data)
	}
	return blobs, nil
}

// GetIDs returns IDs of all Blobs located in DA at given height.
func (s *Service) GetIDs(ctx context.Context, height uint64, namespace da.Namespace) (*da.GetIDsResult, error) {
	var ids []da.ID //nolint:prealloc
	log.Debugw("getting ids", "height", height, "namespace", share.Namespace(namespace))
	blobs, err := s.blobServ.GetAll(ctx, height, []share.Namespace{namespace})
	log.Debugw("got ids", "height", height, "namespace", share.Namespace(namespace))
	if err != nil {
		if strings.Contains(err.Error(), blob.ErrBlobNotFound.Error()) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	for _, b := range blobs {
		ids = append(ids, MakeID(height, b.Commitment))
	}
	h, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}
	return &da.GetIDsResult{IDs: ids, Timestamp: h.Time()}, nil
}

// GetProofs returns inclusion Proofs for all Blobs located in DA at given height.
func (s *Service) GetProofs(ctx context.Context, ids []da.ID, namespace da.Namespace) ([]da.Proof, error) {
	proofs := make([]da.Proof, len(ids))
	for i, id := range ids {
		height, commitment := SplitID(id)
		proof, err := s.blobServ.GetProof(ctx, height, namespace, commitment)
		if err != nil {
			return nil, err
		}
		proofs[i], err = json.Marshal(proof)
		if err != nil {
			return nil, err
		}
	}
	return proofs, nil
}

// Commit creates a Commitment for each given Blob.
func (s *Service) Commit(_ context.Context, daBlobs []da.Blob, namespace da.Namespace) ([]da.Commitment, error) {
	_, commitments, err := s.blobsAndCommitments(daBlobs, namespace)
	return commitments, err
}

// Submit submits the Blobs to Data Availability layer.
func (s *Service) Submit(
	ctx context.Context,
	daBlobs []da.Blob,
	gasPrice float64,
	namespace da.Namespace,
) ([]da.ID, error) {
	return s.SubmitWithOptions(ctx, daBlobs, gasPrice, namespace, nil)
}

// SubmitWithOptions submits the Blobs to Data Availability layer.
func (s *Service) SubmitWithOptions(
	ctx context.Context,
	daBlobs []da.Blob,
	gasPrice float64,
	namespace da.Namespace,
	options []byte,
) ([]da.ID, error) {
	blobs, _, err := s.blobsAndCommitments(daBlobs, namespace)
	if err != nil {
		return nil, err
	}

	opts, err := parseOptions(options)
	if err != nil {
		return nil, err
	}
	opts = append(opts, state.WithGasPrice(gasPrice))
	cfg := state.NewTxConfig(opts...)

	height, err := s.blobServ.Submit(ctx, blobs, cfg)
	if err != nil {
		log.Error("failed to submit blobs", "height", height, "gas price", gasPrice)
		return nil, err
	}
	log.Info("successfully submitted blobs", "height", height, "gas price", gasPrice)
	ids := make([]da.ID, len(blobs))
	for i, blob := range blobs {
		ids[i] = MakeID(height, blob.Commitment)
	}
	return ids, nil
}

func parseOptions(options []byte) ([]state.ConfigOption, error) {
	var opts []state.ConfigOption

	if len(options) == 0 {
		return opts, nil
	}

	parsedOpts := SubmitOptions{}
	err := json.Unmarshal(options, &parsedOpts)
	if err != nil {
		return nil, err
	}

	if parsedOpts.KeyName != "" {
		opts = append(opts, state.WithKeyName(parsedOpts.KeyName))
	}
	if parsedOpts.SignerAddress != "" {
		opts = append(opts, state.WithSignerAddress(parsedOpts.SignerAddress))
	}
	if parsedOpts.FeeGranterAddress != "" {
		opts = append(opts, state.WithFeeGranterAddress(parsedOpts.FeeGranterAddress))
	}
	return opts, nil
}

// blobsAndCommitments converts []da.Blob to []*blob.Blob and generates corresponding
// []da.Commitment
func (s *Service) blobsAndCommitments(
	daBlobs []da.Blob, namespace da.Namespace,
) ([]*blob.Blob, []da.Commitment, error) {
	blobs := make([]*blob.Blob, 0, len(daBlobs))
	commitments := make([]da.Commitment, 0, len(daBlobs))
	for _, daBlob := range daBlobs {
		b, err := blob.NewBlobV0(namespace, daBlob)
		if err != nil {
			return nil, nil, err
		}
		blobs = append(blobs, b)

		commitments = append(commitments, b.Commitment)
	}
	return blobs, commitments, nil
}

// Validate validates Commitments against the corresponding Proofs. This should be possible without
// retrieving the Blobs.
func (s *Service) Validate(
	ctx context.Context,
	ids []da.ID,
	daProofs []da.Proof,
	namespace da.Namespace,
) ([]bool, error) {
	included := make([]bool, len(ids))
	proofs := make([]*blob.Proof, len(ids))
	for i, daProof := range daProofs {
		blobProof := &blob.Proof{}
		err := json.Unmarshal(daProof, blobProof)
		if err != nil {
			return nil, err
		}
		proofs[i] = blobProof
	}
	for i, id := range ids {
		height, commitment := SplitID(id)
		// TODO(tzdybal): for some reason, if proof doesn't match commitment, API returns (false, "blob:
		// invalid proof")    but analysis of the code in celestia-node implies this should never happen -
		// maybe it's caused by openrpc?    there is no way of gently handling errors here, but returned
		// value is fine for us
		fmt.Println("proof", proofs[i] == nil, "commitment", commitment == nil)
		isIncluded, _ := s.blobServ.Included(ctx, height, namespace, proofs[i], commitment)
		included[i] = isIncluded
	}
	return included, nil
}

func MakeID(height uint64, commitment da.Commitment) da.ID {
	id := make([]byte, heightLen+len(commitment))
	binary.LittleEndian.PutUint64(id, height)
	copy(id[heightLen:], commitment)
	return id
}

func SplitID(id da.ID) (uint64, da.Commitment) {
	if len(id) <= heightLen {
		return 0, nil
	}
	commitment := id[heightLen:]
	return binary.LittleEndian.Uint64(id[:heightLen]), commitment
}
