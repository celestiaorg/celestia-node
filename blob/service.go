package blob

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	ErrBlobNotFound = errors.New("blob: not found")
	ErrInvalidProof = errors.New("blob: invalid proof")

	log = logging.Logger("blob")
)

// Submitter is an interface that allows submitting blobs to the celestia-core. It is used to
// avoid a circular dependency between the blob and the state package, since the state package needs
// the blob.Blob type for this signature.
type Submitter interface {
	SubmitPayForBlob(ctx context.Context, fee math.Int, gasLim uint64, blobs []*Blob) (*types.TxResponse, error)
}

type Service struct {
	// accessor dials the given celestia-core endpoint to submit blobs.
	blobSumitter Submitter
	// shareGetter retrieves the EDS to fetch all shares from the requested header.
	shareGetter share.Getter
	//  headerGetter fetches header by the provided height
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error)
}

func NewService(
	submitter Submitter,
	getter share.Getter,
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error),
) *Service {
	return &Service{
		blobSumitter: submitter,
		shareGetter:  getter,
		headerGetter: headerGetter,
	}
}

// Submit sends PFB transaction and reports the height in which it was included.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
func (s *Service) Submit(ctx context.Context, blobs []*Blob) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	var (
		gasLimit = estimateGas(blobs...)
		fee      = int64(appconsts.DefaultMinGasPrice * float64(gasLimit))
	)

	resp, err := s.blobSumitter.SubmitPayForBlob(ctx, types.NewInt(fee), gasLimit, blobs)
	if err != nil {
		return 0, err
	}
	return uint64(resp.Height), nil
}

// Get retrieves all the blobs for given namespaces at the given height by commitment.
func (s *Service) Get(ctx context.Context, height uint64, ns share.Namespace, commitment Commitment) (*Blob, error) {
	blob, _, err := s.getByCommitment(ctx, height, ns, commitment)
	if err != nil {
		return nil, err
	}
	return blob, nil
}

// GetProof retrieves all blobs in the given namespaces at the given height by commitment
// and returns their Proof.
func (s *Service) GetProof(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	commitment Commitment,
) (*Proof, error) {
	_, proof, err := s.getByCommitment(ctx, height, namespace, commitment)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetAll returns all blobs under the given namespaces at the given height.
// GetAll can return blobs and an error in case if some requests failed.
func (s *Service) GetAll(ctx context.Context, height uint64, namespaces []share.Namespace) ([]*Blob, error) {
	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}

	var (
		resultBlobs = make([][]*Blob, len(namespaces))
		resultErr   = make([]error, len(namespaces))
	)

	wg := sync.WaitGroup{}
	for i, namespace := range namespaces {
		wg.Add(1)
		go func(i int, namespace share.Namespace) {
			defer wg.Done()
			blobs, err := s.getBlobs(ctx, namespace, header.DAH)
			if err != nil {
				resultErr[i] = fmt.Errorf("getting blobs for namespace(%s): %s", namespace.String(), err)
				return
			}
			resultBlobs[i] = blobs
		}(i, namespace)
	}
	wg.Wait()

	blobs := make([]*Blob, 0)
	for _, resBlobs := range resultBlobs {
		if len(resBlobs) > 0 {
			blobs = append(blobs, resBlobs...)
		}
	}

	if len(blobs) == 0 {
		resultErr = append(resultErr, ErrBlobNotFound)
	}
	return blobs, errors.Join(resultErr...)
}

// Included verifies that the blob was included in a specific height.
// To ensure that blob was included in a specific height, we need:
// 1. verify the provided commitment by recomputing it;
// 2. verify the provided Proof against subtree roots that were used in 1.;
func (s *Service) Included(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	proof *Proof,
	com Commitment,
) (bool, error) {
	// In the current implementation, LNs will have to download all shares to recompute the commitment.
	// To achieve 1. we need to modify Proof structure and to store all subtree roots, that were
	// involved in commitment creation and then call `merkle.HashFromByteSlices`(tendermint package).
	// nmt.Proof is verifying share inclusion by recomputing row roots, so, theoretically, we can do
	// the same but using subtree roots. For this case, we need an extra method in nmt.Proof
	// that will perform all reconstructions,
	// but we have to guarantee that all our stored subtree roots will be on the same height(e.g. one
	// level above shares).
	// TODO(@vgonkivs): rework the implementation to perform all verification without network requests.
	_, resProof, err := s.getByCommitment(ctx, height, namespace, com)
	switch err {
	case nil:
	case ErrBlobNotFound:
		return false, nil
	default:
		return false, err
	}
	return true, resProof.equal(*proof)
}

// getByCommitment retrieves the DAH row by row, fetching shares and constructing blobs in order to
// compare Commitments. Retrieving is stopped once the requested blob/proof is found.
func (s *Service) getByCommitment(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	commitment Commitment,
) (*Blob, *Proof, error) {
	log.Infow("requesting blob",
		"height", height,
		"namespace", namespace.String())

	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	var (
		rawShares = make([]share.Share, 0)
		proofs    = make(Proof, 0)
		amount    int
		blobShare *shares.Share
	)

	namespacedShares, err := s.shareGetter.GetSharesByNamespace(ctx, header.DAH, namespace)
	if err != nil {
		if errors.Is(err, share.ErrNotFound) {
			err = ErrBlobNotFound
		}
		return nil, nil, err
	}
	for _, row := range namespacedShares {
		if len(row.Shares) == 0 {
			break
		}

		rawShares = append(rawShares, row.Shares...)
		proofs = append(proofs, row.Proof)

		// reconstruct the `blobShare` from the first rawShare in range
		// in order to get blob's length(first share will contain this info)
		if blobShare == nil {
			for i, shr := range rawShares {
				bShare, err := shares.NewShare(shr)
				if err != nil {
					return nil, nil, err
				}

				// ensure that the first share is not a NamespacePaddingShare
				// these shares are used to satisfy the non-interactive default rules
				// and are not the part of the blob, so should be removed.
				isPadding, err := bShare.IsPadding()
				if err != nil {
					return nil, nil, err
				}
				if isPadding {
					continue
				}

				blobShare = bShare
				// save the length.
				length, err := blobShare.SequenceLen()
				if err != nil {
					return nil, nil, err
				}
				amount = shares.SparseSharesNeeded(length)
				rawShares = rawShares[i:]
				break
			}
		}

		// move to the next row if the blob is incomplete.
		if amount > len(rawShares) {
			continue
		}

		blob, same, err := constructAndVerifyBlob(rawShares[:amount], commitment)
		if err != nil {
			return nil, nil, err
		}
		if same {
			return blob, &proofs, nil
		}

		// drop info of the checked blob
		rawShares = rawShares[amount:]
		if len(rawShares) > 0 {
			// save proof for the last row in case we have rawShares
			proofs = proofs[len(proofs)-1:]
		} else {
			// otherwise clear proofs
			proofs = nil
		}
		blobShare = nil
	}

	if len(rawShares) == 0 {
		return nil, nil, ErrBlobNotFound
	}

	blob, same, err := constructAndVerifyBlob(rawShares, commitment)
	if err != nil {
		return nil, nil, err
	}
	if same {
		return blob, &proofs, nil
	}

	return nil, nil, ErrBlobNotFound
}

// getBlobs retrieves the DAH and fetches all shares from the requested Namespace and converts
// them to Blobs.
func (s *Service) getBlobs(ctx context.Context, namespace share.Namespace, root *share.Root) ([]*Blob, error) {
	namespacedShares, err := s.shareGetter.GetSharesByNamespace(ctx, root, namespace)
	if err != nil {
		return nil, err
	}
	return SharesToBlobs(namespacedShares.Flatten())
}
