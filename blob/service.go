package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-app/pkg/shares"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

// TODO(@vgonkivs): remove after bumping celestia-app
// defaultMinGasPrice is the default min gas price.
const defaultMinGasPrice = 0.001

var (
	log = logging.Logger("blob")

	ErrBlobNotFound = errors.New("blob: not found")
)

type Service struct {
	// accessor dials the given celestia-core endpoint to submit blobs.
	accessor *state.CoreAccessor
	// sGetter retrieves the EDS to fetch all shares from the requested header.
	sGetter share.Getter
	// hStore allows to get the requested header from the local storage.
	hStore libhead.Store[*header.ExtendedHeader]
	// headGetter gets current network head.
	headGetter libhead.Head[*header.ExtendedHeader]
}

func NewService(
	state *state.CoreAccessor,
	getter share.Getter,
	hStore libhead.Store[*header.ExtendedHeader],
	headGetter libhead.Head[*header.ExtendedHeader],
) *Service {
	return &Service{
		accessor:   state,
		sGetter:    getter,
		hStore:     hStore,
		headGetter: headGetter,
	}
}

// Submit sends PFB transaction and reports the height it landed on.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
func (s *Service) Submit(ctx context.Context, blobs ...*Blob) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	var (
		gasLimit = estimateGas(blobs...)
		fee      = int64(defaultMinGasPrice * float64(gasLimit))
		b        = make([]*apptypes.Blob, len(blobs))
		err      error
	)

	for i, blob := range blobs {
		b[i], err = apptypes.NewBlob(blob.NamespaceId, blob.Data())
		if err != nil {
			return 0, err
		}
	}

	resp, err := s.accessor.SubmitPayForBlob(ctx, types.NewInt(fee), gasLimit, b...)
	if err != nil {
		return 0, err
	}
	return uint64(resp.Height), nil
}

// Get retrieves all the blobs for given namespaces at the given height by commitment.
func (s *Service) Get(ctx context.Context, height uint64, nID namespace.ID, commitment Commitment) (*Blob, error) {
	blob, _, err := s.getByCommitment(ctx, height, nID, commitment)
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
	nID namespace.ID,
	commitment Commitment,
) (*Proof, error) {
	_, proof, err := s.getByCommitment(ctx, height, nID, commitment)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetAll returns all blobs under the given namespaces at the given height.
// GetAll can return blobs and an error in case if some requests failed.
func (s *Service) GetAll(ctx context.Context, height uint64, nIDs ...namespace.ID) ([]*Blob, error) {
	header, err := s.getByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	var (
		blobs     = make([]*Blob, 0)
		ch        = make(chan []*Blob)
		resultErr = make([]error, len(nIDs))
	)

	for i, nID := range nIDs {
		i := i
		nID := nID
		go func() {
			log.Infow("requesting blobs", "height", height, "nID", nID.String())
			blobs, err := s.getBlobs(ctx, nID, header.DAH)
			if err != nil {
				resultErr[i] = fmt.Errorf("could not get the blob for the nID:%s. %s", nID.String(), err)
				ch <- nil
				return
			}
			ch <- blobs
		}()
	}

	for range nIDs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case b := <-ch:
			if b == nil {
				continue
			}
			blobs = append(blobs, b...)
		}
	}

	if len(blobs) == 0 && len(resultErr) == 0 {
		return nil, ErrBlobNotFound
	}

	err = errors.Join(resultErr...)
	if err != nil {
		err = fmt.Errorf("blob: %s", err)
	}
	return blobs, err
}

// Included verifies that the blob was included in a specific height.
// To ensure that blob was included in a specific height, we need:
// 1. verify the provided commitment by recomputing it;
// 2. verify the provided Proof against subtree roots that were used in 1.;
func (s *Service) Included(
	ctx context.Context,
	height uint64,
	nID namespace.ID,
	_ *Proof,
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
	_, _, err := s.getByCommitment(ctx, height, nID, com)
	if err != nil {
		return false, err
	}

	return true, nil
}

// getByCommitment retrieves the DAH row by row, fetching shares and constructing blobs in order to
// compare Commitments. Retrieving is stopped once the requested blob/proof is found.
func (s *Service) getByCommitment(
	ctx context.Context,
	height uint64,
	nID namespace.ID,
	commitment Commitment,
) (*Blob, *Proof, error) {
	log.Infow("requesting blob",
		"height", height,
		"nID", nID.String())

	header, err := s.getByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	var (
		rawShares = make([]share.Share, 0)
		proofs    = make(Proof, 0)
		amount    int
		blobShare *shares.Share
	)

	namespacedShares, err := s.sGetter.GetSharesByNamespace(ctx, header.DAH, nID)
	if err != nil {
		return nil, nil, err
	}
	for _, rowShares := range namespacedShares {
		if len(rowShares.Shares) == 0 {
			continue
		}

		rawShares = append(rawShares, rowShares.Shares...)
		proofs = append(proofs, rowShares.Proof)

		// reconstruct the `blobShare` from the first rawShare in range
		// in order to get blob's length(first share will contain this info)
		if blobShare == nil {
			for i, sh := range rawShares {
				bShare, err := shares.NewShare(sh)
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

				blobShare = &bShare
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

		// reconstruct the Blob.
		blob, err := sharesToBlobs(rawShares[:amount])
		if err != nil {
			return nil, nil, err
		}

		// compare commitments.
		if bytes.Equal(blob[0].Commitment(), commitment) {
			return blob[0], &proofs, nil
		}

		// drop info of the checked blob
		rawShares = rawShares[amount:]
		blobShare = nil
	}
	return nil, nil, ErrBlobNotFound
}

// getBlobs retrieves the DAH and fetches all shares from the requested namespace.ID and converts
// them to Blobs.
func (s *Service) getBlobs(ctx context.Context, nID namespace.ID, root *share.Root) ([]*Blob, error) {
	namespacedShares, err := s.sGetter.GetSharesByNamespace(ctx, root, nID)
	if err != nil {
		return nil, err
	}
	return sharesToBlobs(namespacedShares.Flatten())
}

// getByHeight returns ExtendedHeader by its height.
// getByHeight ensures that the requested height is less or equal than the current network head.
func (s *Service) getByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	head, err := s.headGetter.Head(ctx)
	if err != nil {
		return nil, err
	}

	if uint64(head.Height()) < height {
		return nil, fmt.Errorf("blob: given height exceeds network head."+
			"networkHeight:%d, requestedHeight:%d", head.Height(), height)
	}

	if uint64(head.Height()) == height {
		return head, nil
	}

	if !s.hStore.HasAt(ctx, height) {
		return nil, fmt.Errorf("blob: given height exceeds local head. requestedHeight:%d", height)
	}

	return s.hStore.GetByHeight(ctx, height)
}
