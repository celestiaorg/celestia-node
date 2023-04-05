package blob

import (
	"bytes"
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-app/pkg/shares"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	log = logging.Logger("blob")

	errBlobNotFound = errors.New("blob: blob not found")
)

type Service struct {
	accessor *state.CoreAccessor
	hGetter  libhead.Getter[*header.ExtendedHeader]
	bGetter  blockservice.BlockGetter
}

func NewService(state *state.CoreAccessor, hGetter libhead.Getter[*header.ExtendedHeader], bGetter blockservice.BlockGetter) *Service {
	return &Service{
		accessor: state,
		hGetter:  hGetter,
		bGetter:  bGetter,
	}
}

func (s *Service) Submit(ctx context.Context, f int64, gasLimit uint64, blobs ...*Blob) (uint64, error) {
	log.Debugw("submitting blobs.", "amount", len(blobs))

	var (
		fee = types.NewInt(f)
		b   = make([]*apptypes.Blob, len(blobs))
		err error
	)

	for i, blob := range blobs {
		b[i], err = apptypes.NewBlob(blob.NamespaceId, blob.Data())
		if err != nil {
			return 0, err
		}
	}

	resp, err := s.accessor.SubmitPayForBlob(ctx, fee, gasLimit, b...)
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

// GetProof retrieves all the blobs for given namespaces at the given height by commitment and return its Proof.
func (s *Service) GetProof(ctx context.Context, height uint64, nID namespace.ID, commitment Commitment) (*Proof, error) {
	_, proof, err := s.getByCommitment(ctx, height, nID, commitment)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetAll returns all found blobs for the given namespaces at the given height.
func (s *Service) GetAll(ctx context.Context, height uint64, nIDs ...namespace.ID) ([]*Blob, error) {
	header, err := s.hGetter.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	cids := rowsToCids(header.DAH.RowsRoots)
	blobs := make([]*Blob, 0)
	ch := make(chan []*Blob, 0)
	for _, nID := range nIDs {
		log.Infow("requesting blobs", "height", height, "nID", nID.String())
		go func(nID namespace.ID) {
			blobs, err := s.getBlobs(ctx, nID, cids)
			if err != nil {
				ch <- nil
				return
			}
			ch <- blobs
		}(nID)
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

	if len(blobs) == 0 {
		return nil, errBlobNotFound
	}
	return blobs, nil
}

// getByCommitment retrieving DAH row by row, fetching shares and constructing blobs in order to compare Commitments.
// Retrieving will be stopped once requested blob/proof will be found.
func (s *Service) getByCommitment(ctx context.Context, height uint64, nID namespace.ID, commitment Commitment) (*Blob, *Proof, error) {
	log.Infow("requesting blob",
		"height", height,
		"nID", nID.String(),
		"commitment", commitment.String())
	header, err := s.hGetter.GetByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	var (
		rawShares = make([]share.Share, 0)
		proofs    = make(Proof, 0)
		blobShare = &shares.Share{}
		seqLen    = uint32(0)
	)

	for _, cid := range rowsToCids(header.DAH.RowsRoots) {
		leaves, proof, err := share.GetSharesByNamespace(ctx, s.bGetter, cid, nID, len(header.DAH.RowsRoots))
		if err != nil {
			return nil, nil, err
		}

		rawShares = append(rawShares, leaves...)
		proofs = append(proofs, proof)
	LOOP:
		for {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}

			// moving to the next round if no shares available
			if len(rawShares) == 0 {
				break LOOP
			}

			// reconstruct the `blobShare` from the first rawShare in range
			// in order to get blob's length(first share will contain this info)
			if blobShare == nil {
				bShare, err := shares.NewShare(rawShares[0])
				if err != nil {
					return nil, nil, err
				}
				blobShare = &bShare
				// save the length.
				seqLen, err = blobShare.SequenceLen()
				if err != nil {
					return nil, nil, err
				}
			}

			// move to the next row if the blob is incomplete.
			if int(seqLen) > len(rawShares) {
				break LOOP
			}

			// reconstruct the Blob.
			blob, err := sharesToBlobs(rawShares[:seqLen])
			if err != nil {
				return nil, nil, err
			}

			// compare commitments.
			if bytes.Equal(blob[0].Commitment(), commitment) {
				return blob[0], &proofs, nil
			}

			// drop info of the checked blob
			rawShares = rawShares[seqLen:]
			blobShare = nil
		}
	}
	return nil, nil, errBlobNotFound
}

// Included verifies that the blob was included in a specific height.
// To ensure that blob was included in a specific height, we need:
// 1. verify the provided commitment by recomputing it;
// 2. verify the provided Proof against subtree roots that were used in 1.;
func (s *Service) Included(ctx context.Context, height uint64, nID namespace.ID, _ *Proof, com Commitment) (bool, error) {
	// In the current implementation, LNs will have to download all shares to recompute the commitment.
	// To achieve 1. we need to modify Proof structure and to store all subtree roots, that were involved in
	// commitment creation and then call
	// https://github.com/tendermint/tendermint/blob/35581cf54ec436b8c37fabb43fdaa3f48339a170/crypto/merkle/tree.go#L9;
	// nmt.Proof is verifying share inclusion by recomputing row roots, so, theoretically, we can do the same but using
	// subtree roots. For this case, we need an extra method in nmt.Proof that will perform all reconstructions, but we have
	// to guarantee that all our stored subtree roots will be on the same height(e.g. one level above shares)
	// TODO(@vgonkivs): rework the implementation to perform all verification without network requests.
	_, _, err := s.getByCommitment(ctx, height, nID, com)
	if err != nil {
		return false, err
	}

	return true, nil
}

// getBlobs retrieves the whole DAH and fetches all shares from the requested namespace.ID and converts them to Blobs.
func (s *Service) getBlobs(ctx context.Context, nID namespace.ID, cids []cid.Cid) ([]*Blob, error) {
	rawShares := make([]share.Share, 0)
	for _, cid := range cids {
		shares, _, err := share.GetSharesByNamespace(ctx, s.bGetter, cid, nID, len(cids))
		if err != nil {
			return nil, err
		}

		if len(shares) == 0 {
			break
		}

		rawShares = append(rawShares, shares...)
	}
	return sharesToBlobs(rawShares)
}
