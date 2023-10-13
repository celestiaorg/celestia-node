package blob

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"

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
	blobSubmitter Submitter
	// shareGetter retrieves the EDS to fetch all shares from the requested header.
	shareGetter share.Getter
	// headerGetter fetches header by the provided height
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error)
}

func NewService(
	submitter Submitter,
	getter share.Getter,
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error),
) *Service {
	return &Service{
		blobSubmitter: submitter,
		shareGetter:   getter,
		headerGetter:  headerGetter,
	}
}

// SubmitOptions contains the information about fee and gasLimit price in order to configure the
// Submit request.
type SubmitOptions struct {
	Fee      int64
	GasLimit uint64
}

// DefaultSubmitOptions creates a default fee and gas price values.
func DefaultSubmitOptions() *SubmitOptions {
	return &SubmitOptions{
		Fee:      -1,
		GasLimit: 0,
	}
}

// Submit sends PFB transaction and reports the height in which it was included.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
// Handles gas estimation and fee calculation.
func (s *Service) Submit(ctx context.Context, blobs []*Blob, options *SubmitOptions) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	if options == nil {
		options = DefaultSubmitOptions()
	}

	resp, err := s.blobSubmitter.SubmitPayForBlob(ctx, types.NewInt(options.Fee), options.GasLimit, blobs)
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
			blobs, err := s.getBlobs(ctx, namespace, header)
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

	namespacedShares, err := s.shareGetter.GetSharesByNamespace(ctx, header, namespace)
	if err != nil {
		if errors.Is(err, share.ErrNotFound) {
			err = ErrBlobNotFound
		}
		return nil, nil, err
	}

	var (
		rawShares = make([]shares.Share, 0)
		proofs    = make(Proof, 0)
		// spansMultipleRows specifies whether blob is expanded into multiple rows
		spansMultipleRows bool
	)

	for _, row := range namespacedShares {
		appShares, err := toAppShares(row.Shares...)
		if err != nil {
			return nil, nil, err
		}
		rawShares = append(rawShares, appShares...)
		proofs = append(proofs, row.Proof)

		var blobs []*Blob
		blobs, rawShares, err = buildBlobsIfExist(rawShares)
		if err != nil {
			return nil, nil, err
		}
		for _, b := range blobs {
			if b.Commitment.Equal(commitment) {
				return b, &proofs, nil
			}
			// Falling under this flag means that the data from the last row
			// was insufficient to create a complete blob. As a result,
			// the first blob received spans two rows and includes proofs
			// for both of these rows. All other blobs in the result will relate
			// to the current row and have a single proof.
			if spansMultipleRows {
				spansMultipleRows = false
				// leave proof only for the current row
				proofs = proofs[len(proofs)-1:]
			}
		}

		if len(rawShares) > 0 {
			spansMultipleRows = true
			continue
		}
		proofs = nil
	}

	err = ErrBlobNotFound
	if len(rawShares) > 0 {
		err = fmt.Errorf("incomplete blob with the "+
			"namespace: %s detected at %d: %w", namespace.String(), height, err)
		log.Error(err)
	}
	return nil, nil, err
}

// getBlobs retrieves the DAH and fetches all shares from the requested Namespace and converts
// them to Blobs.
func (s *Service) getBlobs(
	ctx context.Context,
	namespace share.Namespace,
	header *header.ExtendedHeader,
) ([]*Blob, error) {
	namespacedShares, err := s.shareGetter.GetSharesByNamespace(ctx, header, namespace)
	if err != nil {
		return nil, err
	}
	return SharesToBlobs(namespacedShares.Flatten())
}

// toAppShares converts node's raw shares to the app shares, skipping padding
func toAppShares(shrs ...share.Share) ([]shares.Share, error) {
	appShrs := make([]shares.Share, 0, len(shrs))
	for _, shr := range shrs {
		bShare, err := shares.NewShare(shr)
		if err != nil {
			return nil, err
		}

		ok, err := bShare.IsPadding()
		if err != nil {
			return nil, err
		}
		if ok {
			continue
		}

		appShrs = append(appShrs, *bShare)
	}
	return appShrs, nil
}
