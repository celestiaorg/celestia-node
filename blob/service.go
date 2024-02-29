package blob

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth/types"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	ErrBlobNotFound = errors.New("blob: not found")
	ErrInvalidProof = errors.New("blob: invalid proof")

	log    = logging.Logger("blob")
	tracer = otel.Tracer("blob/service")
)

// GasPrice represents the amount to be paid per gas unit. Fee is set by
// multiplying GasPrice by GasLimit, which is determined by the blob sizes.
type GasPrice float64

// DefaultGasPrice returns the default gas price, letting node automatically
// determine the Fee based on the passed blob sizes.
func DefaultGasPrice() GasPrice {
	return -1.0
}

// Submitter is an interface that allows submitting blobs to the celestia-core. It is used to
// avoid a circular dependency between the blob and the state package, since the state package needs
// the blob.Blob type for this signature.
type Submitter interface {
	SubmitPayForBlob(ctx context.Context, fee sdkmath.Int, gasLim uint64, blobs []*Blob) (*types.TxResponse, error)
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

// Submit sends PFB transaction and reports the height at which it was included.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
// Handles gas estimation and fee calculation.
func (s *Service) Submit(ctx context.Context, blobs []*Blob, gasPrice GasPrice) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	options := DefaultSubmitOptions()
	if gasPrice >= 0 {
		blobSizes := make([]uint32, len(blobs))
		for i, blob := range blobs {
			blobSizes[i] = uint32(len(blob.Data))
		}
		options.GasLimit = blobtypes.EstimateGas(blobSizes, appconsts.DefaultGasPerBlobByte, auth.DefaultTxSizeCostPerByte)
		options.Fee = types.NewInt(int64(math.Ceil(float64(gasPrice) * float64(options.GasLimit)))).Int64()
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

	for _, ns := range namespaces {
		log.Debugw("performing GetAll request", "namespace", ns.String(), "height", height)
	}

	wg := sync.WaitGroup{}
	for i, namespace := range namespaces {
		wg.Add(1)
		go func(i int, namespace share.Namespace) {
			defer wg.Done()
			blobs, err := s.getBlobs(ctx, namespace, header)
			if err != nil {
				resultErr[i] = fmt.Errorf("getting blobs for namespace(%s): %w", namespace.String(), err)
				return
			}

			log.Debugw("receiving blobs", "height", height, "total", len(blobs))
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
) (_ bool, err error) {
	ctx, span := tracer.Start(ctx, "included")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
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
) (_ *Blob, _ *Proof, err error) {
	log.Infow("requesting blob",
		"height", height,
		"namespace", namespace.String())

	ctx, span := tracer.Start(ctx, "get-by-commitment")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("commitment", string(commitment)),
	)

	getCtx, headerGetterSpan := tracer.Start(ctx, "header-getter")

	header, err := s.headerGetter(getCtx, height)
	if err != nil {
		headerGetterSpan.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	headerGetterSpan.SetStatus(codes.Ok, "")
	headerGetterSpan.AddEvent("received eds", trace.WithAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots)))))

	getCtx, getSharesSpan := tracer.Start(ctx, "get-shares-by-namespace")

	namespacedShares, err := s.shareGetter.GetSharesByNamespace(getCtx, header, namespace)
	if err != nil {
		if errors.Is(err, share.ErrNotFound) {
			err = ErrBlobNotFound
		}
		getSharesSpan.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	getSharesSpan.SetStatus(codes.Ok, "")
	getSharesSpan.AddEvent("received shares", trace.WithAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots)))))

	var (
		rawShares = make([]shares.Share, 0)
		proofs    = make(Proof, 0)
		// spansMultipleRows specifies whether blob is expanded into multiple rows
		spansMultipleRows bool
	)

	for _, row := range namespacedShares {
		if len(row.Shares) == 0 {
			// the above condition means that we've faced with an Absence Proof.
			// This Proof proves that the namespace was not found in the DAH, so
			// we can return `ErrBlobNotFound`.
			return nil, nil, ErrBlobNotFound
		}

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
				span.AddEvent("blob reconstructed")
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
) (_ []*Blob, err error) {
	ctx, span := tracer.Start(ctx, "get-blobs")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
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
