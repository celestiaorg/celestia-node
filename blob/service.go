package blob

import (
	bytes2 "bytes"
	"context"
	"encoding/hex"
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
	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	pkgproof "github.com/celestiaorg/celestia-app/pkg/proof"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

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
// Get collects all namespaced data from the EDS, constructs blobs
// and compares commitments. `ErrBlobNotFound` can be returned in case blob was not found.
func (s *Service) Get(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	commitment Commitment,
) (blob *Blob, err error) {
	ctx, span := tracer.Start(ctx, "get")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)

	sharesParser := &parser{verifyFn: func(blob *Blob) bool {
		return blob.compareCommitments(commitment)
	}}

	blob, _, err = s.retrieve(ctx, height, namespace, sharesParser)
	return
}

// GetProof retrieves all blobs in the given namespaces at the given height by commitment
// and returns their Proof. It collects all namespaced data from the EDS, constructs blobs
// and compares commitments.
func (s *Service) GetProof(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	commitment Commitment,
) (proof *Proof, err error) {
	ctx, span := tracer.Start(ctx, "get-proof")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)

	sharesParser := &parser{verifyFn: func(blob *Blob) bool {
		return blob.compareCommitments(commitment)
	}}

	_, proof, err = s.retrieve(ctx, height, namespace, sharesParser)
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

	for _, namespace := range namespaces {
		log.Debugw("performing GetAll request", "namespace", namespace.String(), "height", height)
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
	commitment Commitment,
) (_ bool, err error) {
	ctx, span := tracer.Start(ctx, "included")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)

	// In the current implementation, LNs will have to download all shares to recompute the commitment.
	// TODO(@vgonkivs): rework the implementation to perform all verification without network requests.
	sharesParser := &parser{verifyFn: func(blob *Blob) bool {
		return blob.compareCommitments(commitment)
	}}
	_, resProof, err := s.retrieve(ctx, height, namespace, sharesParser)
	switch {
	case err == nil:
	case errors.Is(err, ErrBlobNotFound):
		return false, nil
	default:
		return false, err
	}
	return true, resProof.equal(*proof)
}

// retrieve retrieves blobs and their proofs by requesting the whole namespace and
// comparing Commitments.
// Retrieving is stopped once the `verify` condition in shareParser is met.
func (s *Service) retrieve(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	sharesParser *parser,
) (_ *Blob, _ *Proof, err error) {
	log.Infow("requesting blob",
		"height", height,
		"namespace", namespace.String())

	getCtx, headerGetterSpan := tracer.Start(ctx, "header-getter")

	header, err := s.headerGetter(getCtx, height)
	if err != nil {
		headerGetterSpan.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	headerGetterSpan.SetStatus(codes.Ok, "")
	headerGetterSpan.AddEvent("received eds", trace.WithAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots)))))

	rowIndex := -1
	for i, row := range header.DAH.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowIndex = i
			break
		}
	}

	getCtx, getSharesSpan := tracer.Start(ctx, "get-shares-by-namespace")

	// collect shares for the requested namespace
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
		appShares = make([]shares.Share, 0)
		proofs    = make(Proof, 0)
	)

	for _, row := range namespacedShares {
		if len(row.Shares) == 0 {
			// the above condition means that we've faced with an Absence Proof.
			// This Proof proves that the namespace was not found in the DAH, so
			// we can return `ErrBlobNotFound`.
			return nil, nil, ErrBlobNotFound
		}

		appShares, err = toAppShares(row.Shares...)
		if err != nil {
			return nil, nil, err
		}

		proofs = append(proofs, row.Proof)
		index := row.Proof.Start()

		for {
			var (
				isComplete bool
				shrs       []shares.Share
				wasEmpty   = sharesParser.isEmpty()
			)

			if wasEmpty {
				// create a parser if it is empty
				shrs, err = sharesParser.set(rowIndex*len(header.DAH.RowRoots)+index, appShares)
				if err != nil {
					if errors.Is(err, errEmptyShares) {
						// reset parser as `skipPadding` can update next blob's index
						sharesParser.reset()
						appShares = nil
						break
					}
					return nil, nil, err
				}

				// update index and shares if padding shares were detected.
				if len(appShares) != len(shrs) {
					index += len(appShares) - len(shrs)
					appShares = shrs
				}
			}

			shrs, isComplete = sharesParser.addShares(appShares)
			// move to the next row if the blob is incomplete
			if !isComplete {
				appShares = nil
				break
			}
			// otherwise construct blob
			blob, err := sharesParser.parse()
			if err != nil {
				return nil, nil, err
			}

			if sharesParser.verify(blob) {
				return blob, &proofs, nil
			}

			index += len(appShares) - len(shrs)
			appShares = shrs
			sharesParser.reset()

			if !wasEmpty {
				// remove proofs for prev rows if verified blob spans multiple rows
				proofs = proofs[len(proofs)-1:]
			}
		}

		rowIndex++
		if sharesParser.isEmpty() {
			proofs = nil
		}
	}

	err = ErrBlobNotFound
	for _, sh := range appShares {
		ok, err := sh.IsPadding()
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			err = fmt.Errorf("incomplete blob with the "+
				"namespace: %s detected at %d: %w", namespace.String(), height, err)
			log.Error(err)
		}
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
	span.SetAttributes(
		attribute.Int64("height", int64(header.Height())),
		attribute.String("namespace", namespace.String()),
	)
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	blobs := make([]*Blob, 0)
	verifyFn := func(blob *Blob) bool {
		blobs = append(blobs, blob)
		return false
	}
	sharesParser := &parser{verifyFn: verifyFn}

	_, _, err = s.retrieve(ctx, header.Height(), namespace, sharesParser)
	if len(blobs) == 0 {
		return nil, ErrBlobNotFound
	}
	return blobs, nil
}

func (s *Service) GetCommitmentProof(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	shareCommitment []byte,
) (*CommitmentProof, error) {
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
	blb, err := s.Get(ctx, height, namespace, shareCommitment)
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
	blobShares, err := BlobsToShares(blb)
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
	extendedHeader, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}

	log.Debugw("getting eds", "height", height)
	eds, err := s.shareGetter.GetEDS(ctx, extendedHeader)
	if err != nil {
		return nil, err
	}

	return ProveCommitment(eds, namespace, blobShares)
}

func ProveCommitment(
	eds *rsmt2d.ExtendedDataSquare,
	namespace share.Namespace,
	blobShares []share.Share,
) (*CommitmentProof, error) {
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
		"start_share",
		blobSharesStartIndex,
		"end_share",
		blobSharesStartIndex+len(blobShares),
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
	log.Debugw("computing the subtree roots")
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

	log.Debugw("successfully proved the share commitment")
	commitmentProof := CommitmentProof{
		SubtreeRoots:      subtreeRoots,
		SubtreeRootProofs: nmtProofs,
		NamespaceID:       namespace.ID(),
		RowProof:          sharesProof.RowProof,
		NamespaceVersion:  namespace.Version(),
	}
	return &commitmentProof, nil
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
