package blob

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/celestiaorg/go-square/inclusion"
	"slices"
	"sync"

	"github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/libs/bytes"
	core "github.com/tendermint/tendermint/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	pkgproof "github.com/celestiaorg/celestia-app/v2/pkg/proof"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/nmt"
)

var (
	ErrBlobNotFound       = errors.New("blob: not found")
	ErrInvalidProof       = errors.New("blob: invalid proof")
	ErrMismatchCommitment = errors.New("blob: mismatched commitment")

	log    = logging.Logger("blob")
	tracer = otel.Tracer("blob/service")
)

// SubmitOptions aliases TxOptions from state package allowing users
// to specify options for SubmitPFB transaction.
type SubmitOptions = state.TxConfig

// Submitter is an interface that allows submitting blobs to the celestia-core. It is used to
// avoid a circular dependency between the blob and the state package, since the state package needs
// the blob.Blob type for this signature.
type Submitter interface {
	SubmitPayForBlob(context.Context, []*state.Blob, *state.TxConfig) (*types.TxResponse, error)
}

type Service struct {
	// ctx represents the Service's lifecycle context.
	ctx    context.Context
	cancel context.CancelFunc
	// accessor dials the given celestia-core endpoint to submit blobs.
	blobSubmitter Submitter
	// shareGetter retrieves the EDS to fetch all shares from the requested header.
	shareGetter shwap.Getter
	// headerGetter fetches header by the provided height
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error)
	// headerSub subscribes to new headers to supply to blob subscriptions.
	headerSub func(ctx context.Context) (<-chan *header.ExtendedHeader, error)
}

func NewService(
	submitter Submitter,
	getter shwap.Getter,
	headerGetter func(context.Context, uint64) (*header.ExtendedHeader, error),
	headerSub func(ctx context.Context) (<-chan *header.ExtendedHeader, error),
) *Service {
	return &Service{
		blobSubmitter: submitter,
		shareGetter:   getter,
		headerGetter:  headerGetter,
		headerSub:     headerSub,
	}
}

func (s *Service) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *Service) Stop(context.Context) error {
	s.cancel()
	return nil
}

// SubscriptionResponse is the response type for the Subscribe method.
// It contains the blobs and the height at which they were included.
// If the Blobs slice is empty, it means that no blobs were included at the given height.
type SubscriptionResponse struct {
	Blobs  []*Blob
	Height uint64
}

// Subscribe returns a channel that will receive SubscriptionResponse objects.
// The channel will be closed when the context is canceled or the service is stopped.
// Please note that no errors are returned: underlying operations are retried until successful.
// Additionally, not reading from the returned channel will cause the stream to close after 16 messages.
func (s *Service) Subscribe(ctx context.Context, ns share.Namespace) (<-chan *SubscriptionResponse, error) {
	if s.ctx == nil {
		return nil, fmt.Errorf("service has not been started")
	}

	headerCh, err := s.headerSub(ctx)
	if err != nil {
		return nil, err
	}

	blobCh := make(chan *SubscriptionResponse, 16)
	go func() {
		defer close(blobCh)

		for {
			select {
			case header, ok := <-headerCh:
				if ctx.Err() != nil {
					log.Debugw("blobsub: canceling subscription due to user ctx closing", "namespace", ns.ID())
					return
				}
				if !ok {
					log.Errorw("header channel closed for subscription", "namespace", ns.ID())
					return
				}
				// close subscription before buffer overflows
				if len(blobCh) == cap(blobCh) {
					log.Debugw("blobsub: canceling subscription due to buffer overflow from slow reader", "namespace", ns.ID())
					return
				}

				var blobs []*Blob
				var err error
				for {
					blobs, err = s.getAll(ctx, header, []share.Namespace{ns})
					if ctx.Err() != nil {
						// context canceled, continuing would lead to unexpected missed heights for the client
						log.Debugw("blobsub: canceling subscription due to user ctx closing", "namespace", ns.ID())
						return
					}
					if err == nil {
						// operation successful, break the loop
						break
					}
				}

				select {
				case <-ctx.Done():
					log.Debugw("blobsub: pending response canceled due to user ctx closing", "namespace", ns.ID())
					return
				case blobCh <- &SubscriptionResponse{Blobs: blobs, Height: header.Height()}:
				}
			case <-ctx.Done():
				log.Debugw("blobsub: canceling subscription due to user ctx closing", "namespace", ns.ID())
				return
			case <-s.ctx.Done():
				log.Debugw("blobsub: canceling subscription due to service ctx closing", "namespace", ns.ID())
				return
			}
		}
	}()
	return blobCh, nil
}

// Submit sends PFB transaction and reports the height at which it was included.
// Allows sending multiple Blobs atomically synchronously.
// Uses default wallet registered on the Node.
// Handles gas estimation and fee calculation.
func (s *Service) Submit(ctx context.Context, blobs []*Blob, txConfig *SubmitOptions) (uint64, error) {
	log.Debugw("submitting blobs", "amount", len(blobs))

	squareBlobs := make([]*state.Blob, len(blobs))
	for i := range blobs {
		if err := blobs[i].Namespace().ValidateForBlob(); err != nil {
			return 0, err
		}
		squareBlobs[i] = blobs[i].Blob
	}

	resp, err := s.blobSubmitter.SubmitPayForBlob(ctx, squareBlobs, txConfig)
	if err != nil {
		return 0, err
	}
	return uint64(resp.Height), nil
}

// Get retrieves a blob in a given namespace at the given height by commitment.
// Get collects all namespaced data from the EDS, construct the blob
// and compares the commitment argument.
// `ErrBlobNotFound` can be returned in case blob was not found.
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

// GetProof returns an NMT inclusion proof for a specified namespace to the respective row roots
// on which the blob spans on at the given height, using the given commitment.
// It employs the same algorithm as service.Get() internally.
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

	_, proof, err = s.retrieveBlobProof(ctx, height, namespace, sharesParser)
	return proof, err
}

// GetAll returns all blobs under the given namespaces at the given height.
// If all blobs were found without any errors, the user will receive a list of blobs.
// If the BlobService couldn't find any blobs under the requested namespaces,
// the user will receive an empty list of blobs along with an empty error.
// If some of the requested namespaces were not found, the user will receive all the found blobs
// and an empty error. If there were internal errors during some of the requests,
// the user will receive all found blobs along with a combined error message.
//
// All blobs will preserve the order of the namespaces that were requested.
func (s *Service) GetAll(ctx context.Context, height uint64, namespaces []share.Namespace) ([]*Blob, error) {
	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}

	return s.getAll(ctx, header, namespaces)
}

func (s *Service) getAll(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespaces []share.Namespace,
) ([]*Blob, error) {
	height := header.Height()
	var (
		resultBlobs = make([][]*Blob, len(namespaces))
		resultErr   = make([]error, len(namespaces))
		wg          = sync.WaitGroup{}
	)
	for i, namespace := range namespaces {
		wg.Add(1)
		go func(i int, namespace share.Namespace) {
			log.Debugw("retrieving all blobs from", "namespace", namespace.String(), "height", height)
			defer wg.Done()

			blobs, err := s.getBlobs(ctx, namespace, header)
			if err != nil && !errors.Is(err, ErrBlobNotFound) {
				log.Errorf("getting blobs for namespaceID(%s): %v", namespace.ID().String(), err)
				resultErr[i] = err
			}
			if len(blobs) > 0 {
				log.Infow("retrieved blobs", "height", height, "total", len(blobs))
				resultBlobs[i] = blobs
			}
		}(i, namespace)
	}
	wg.Wait()

	blobs := slices.Concat(resultBlobs...)
	err := errors.Join(resultErr...)
	return blobs, err
}

// Included verifies that the blob was included in a specific height.
// To ensure that blob was included in a specific height, we need:
// 1. verify the provided commitment by recomputing it;
// 2. verify the provided Proof against subtree roots that were used in 1.;
// Note: this method can be deprecated because it's doing processing that can
// be done locally.
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
	// verify that the blob subtree roots match the proof subtree roots
	if proofCommitment := proof.GenerateCommitment(); !commitment.Equal(proofCommitment) {
		return false, fmt.Errorf(
			`unequal blob commitment %s and proof commitment %s`,
			hex.EncodeToString(commitment),
			hex.EncodeToString(proofCommitment),
		)
	}
	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return false, err
	}
	return proof.Verify(header.DataHash, appconsts.SubtreeRootThreshold(appVersion))
}

// retrieve retrieves blobs and their proofs by requesting the whole namespace and
// comparing Commitments.
// Retrieving is stopped once the `verify` condition in shareParser is met.
func (s *Service) retrieve(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	sharesParser *parser,
) (_ *Blob, _ *namespaceToRowRootProof, err error) {
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
	headerGetterSpan.AddEvent("received header", trace.WithAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots)))))

	rowIndex := -1
	for i, row := range header.DAH.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			rowIndex = i
			break
		}
	}

	// Note: there is no need to check whether the row index is different from -1
	// because it will be handled at the end to return the correct error.

	getCtx, getSharesSpan := tracer.Start(ctx, "get-shares-by-namespace")

	// collect shares for the requested namespace
	namespacedShares, err := s.shareGetter.GetSharesByNamespace(getCtx, header, namespace)
	if err != nil {
		if errors.Is(err, shwap.ErrNotFound) {
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
		proofs    = make(namespaceToRowRootProof, 0)
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

// retrieve retrieves blobs and their proofs by requesting the whole namespace and
// comparing Commitments.
// Retrieving is stopped once the `verify` condition in shareParser is met.
func (s *Service) retrieveBlobProof(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	sharesParser *parser,
) (_ *Blob, _ *Proof, err error) {
	log.Infow("requesting blob proof",
		"height", height,
		"namespace", namespace.String())

	getCtx, headerGetterSpan := tracer.Start(ctx, "header-getter")

	header, err := s.headerGetter(getCtx, height)
	if err != nil {
		headerGetterSpan.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	// find the index of the row where the blob could start
	inclusiveNamespaceStartRowIndex := -1
	for i, row := range header.DAH.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			inclusiveNamespaceStartRowIndex = i
			break
		}
	}
	if inclusiveNamespaceStartRowIndex == -1 {
		return nil, nil, ErrBlobNotFound
	}

	// end exclusive index of the row root containing the namespace
	exclusiveNamespaceEndRowIndex := inclusiveNamespaceStartRowIndex
	for _, row := range header.DAH.RowRoots[inclusiveNamespaceStartRowIndex:] {
		if namespace.IsOutsideRange(row, row) {
			break
		}
		exclusiveNamespaceEndRowIndex++
	}
	if exclusiveNamespaceEndRowIndex == inclusiveNamespaceStartRowIndex {
		return nil, nil, fmt.Errorf("couldn't find the row index of the namespace end")
	}

	eds, err := s.shareGetter.GetEDS(ctx, header)
	if err != nil {
		headerGetterSpan.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}
	headerGetterSpan.SetStatus(codes.Ok, "")
	headerGetterSpan.AddEvent("received eds", trace.WithAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots)))))

	// calculate the square size
	squareSize := len(header.DAH.RowRoots) / 2

	// get all the shares of the rows containing the namespace
	_, getSharesSpan := tracer.Start(ctx, "get-all-shares-in-namespace")
	// store the ODS shares of the rows containing the blob
	odsShares := make([]share.Share, 0, (exclusiveNamespaceEndRowIndex-inclusiveNamespaceStartRowIndex)*squareSize)
	// store the EDS shares of the rows containing the blob
	edsShares := make([][]shares.Share, exclusiveNamespaceEndRowIndex-inclusiveNamespaceStartRowIndex)

	for rowIndex := inclusiveNamespaceStartRowIndex; rowIndex < exclusiveNamespaceEndRowIndex; rowIndex++ {
		rowShares := eds.Row(uint(rowIndex))
		odsShares = append(odsShares, rowShares[:squareSize]...)
		rowAppShares, err := toAppShares(rowShares...)
		if err != nil {
			return nil, nil, err
		}
		edsShares[rowIndex-inclusiveNamespaceStartRowIndex] = rowAppShares
	}

	getSharesSpan.SetStatus(codes.Ok, "")
	getSharesSpan.AddEvent("received shares", trace.WithAttributes(
		attribute.Int64("eds-size", int64(squareSize*2))))

	// go over the shares until finding the requested blobs
	for currentShareIndex := 0; currentShareIndex < len(odsShares); {
		currentShareApp, err := shares.NewShare(odsShares[currentShareIndex])
		if err != nil {
			return nil, nil, err
		}

		// skip if it's a padding share
		isPadding, err := currentShareApp.IsPadding()
		if err != nil {
			return nil, nil, err
		}
		if isPadding {
			currentShareIndex++
			continue
		}
		isCompactShare, err := currentShareApp.IsCompactShare()
		if err != nil {
			return nil, nil, err
		}
		if isCompactShare {
			currentShareIndex++
			continue
		}
		isSequenceStart, err := currentShareApp.IsSequenceStart()
		if err != nil {
			return nil, nil, err
		}
		if isSequenceStart {
			// calculate the blob length
			sequenceLength, err := currentShareApp.SequenceLen()
			if err != nil {
				return nil, nil, err
			}
			blobLen := shares.SparseSharesNeeded(sequenceLength)

			exclusiveEndShareIndex := currentShareIndex + blobLen
			if exclusiveEndShareIndex > len(odsShares) {
				// this blob spans to the next row which has a namespace > requested namespace.
				// this means that we can stop.
				return nil, nil, ErrBlobNotFound
			}
			// convert the blob shares to app shares
			blobShares := odsShares[currentShareIndex:exclusiveEndShareIndex]
			appBlobShares, err := toAppShares(blobShares...)
			if err != nil {
				return nil, nil, err
			}

			// parse the blob
			sharesParser.length = blobLen
			_, isComplete := sharesParser.addShares(appBlobShares)
			if !isComplete {
				return nil, nil, fmt.Errorf("expected the shares to construct a full blob")
			}
			blob, err := sharesParser.parse()
			if err != nil {
				return nil, nil, err
			}

			// number of shares per EDS row
			numberOfSharesPerEDSRow := squareSize * 2
			// number of shares from square start to namespace start
			sharesFromSquareStartToNsStart := inclusiveNamespaceStartRowIndex * numberOfSharesPerEDSRow
			// number of rows from namespace start row to current row
			rowsFromNsStartToCurrentRow := currentShareIndex / squareSize
			// number of shares from namespace row start to current row
			sharesFromNsStartToCurrentRow := rowsFromNsStartToCurrentRow * numberOfSharesPerEDSRow
			// number of shares from the beginning of current row to current share
			sharesFromCurrentRowStart := currentShareIndex % squareSize
			// setting the index manually since we didn't use the parser.set() method
			blob.index = sharesFromSquareStartToNsStart +
				sharesFromNsStartToCurrentRow +
				sharesFromCurrentRowStart

			if blob.Namespace().Equals(namespace) && sharesParser.verify(blob) {
				// now that we found the requested blob, we will create
				// its inclusion proof.
				inclusiveBlobStartRowIndex := blob.index / (squareSize * 2)
				exclusiveBlobEndRowIndex := inclusiveNamespaceStartRowIndex + exclusiveEndShareIndex/squareSize
				if (currentShareIndex+blobLen)%squareSize != 0 {
					// if the row is not complete with the blob shares,
					// then we increment the exclusive blob end row index
					// so that it's exclusive.
					exclusiveBlobEndRowIndex++
				}

				// create the row roots to data root inclusion proof
				rowProofs := proveRowRootsToDataRoot(
					append(header.DAH.RowRoots, header.DAH.ColumnRoots...),
					inclusiveBlobStartRowIndex,
					exclusiveBlobEndRowIndex,
				)
				rowRoots := make([]bytes.HexBytes, exclusiveBlobEndRowIndex-inclusiveBlobStartRowIndex)
				for index, rowRoot := range header.DAH.RowRoots[inclusiveBlobStartRowIndex:exclusiveBlobEndRowIndex] {
					rowRoots[index] = rowRoot
				}

				edsShareStart := inclusiveBlobStartRowIndex - inclusiveNamespaceStartRowIndex
				edsShareEnd := exclusiveBlobEndRowIndex - inclusiveNamespaceStartRowIndex
				// create the share to row root proofs
				shareToRowRootProofs, _, err := pkgproof.CreateShareToRowRootProofs(
					squareSize,
					edsShares[edsShareStart:edsShareEnd],
					header.DAH.RowRoots[inclusiveBlobStartRowIndex:exclusiveBlobEndRowIndex],
					currentShareIndex%squareSize,
					(exclusiveEndShareIndex-1)%squareSize,
				)
				if err != nil {
					return nil, nil, err
				}

				// convert the share to row root proof to an nmt.Proof
				nmtShareToRowRootProofs := toNMTProof(shareToRowRootProofs)

				subtreeRoots, err := inclusion.GenerateSubtreeRoots(blob.Blob, appconsts.SubtreeRootThreshold(appVersion))
				if err != nil {
					return nil, nil, err
				}

				proof := Proof{
					SubtreeRootProofs: nmtShareToRowRootProofs,
					RowToDataRootProof: core.RowProof{
						RowRoots: rowRoots,
						Proofs:   rowProofs,
						StartRow: uint32(inclusiveBlobStartRowIndex),
						EndRow:   uint32(exclusiveBlobEndRowIndex) - 1,
					},
					SubtreeRoots: subtreeRoots,
				}
				return blob, &proof, nil
			}
			sharesParser.reset()
			currentShareIndex += blobLen
		} else {
			// this is a continuation of a previous blob
			// we can skip
			currentShareIndex++
		}
	}
	return nil, nil, ErrBlobNotFound
}

func toNMTProof(proofs []*pkgproof.NMTProof) []*nmt.Proof {
	nmtShareToRowRootProofs := make([]*nmt.Proof, 0, len(proofs))
	for _, proof := range proofs {
		nmtProof := nmt.NewInclusionProof(int(proof.Start), int(proof.End), proof.Nodes, true)
		nmtShareToRowRootProofs = append(nmtShareToRowRootProofs, &nmtProof)
	}
	return nmtShareToRowRootProofs
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
	return blobs, err
}
