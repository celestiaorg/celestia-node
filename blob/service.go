package blob

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
	pkgproof "github.com/celestiaorg/celestia-app/v6/pkg/proof"
	"github.com/celestiaorg/go-square/v3/inclusion"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	ErrBlobNotFound = errors.New("blob: not found")
	ErrInvalidProof = errors.New("blob: invalid proof")

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
	SubmitPayForBlob(context.Context, []*libshare.Blob, *state.TxConfig) (*types.TxResponse, error)
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
	// metrics tracks blob-related metrics
	metrics *metrics
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
		metrics:       nil, // Will be initialized via WithMetrics() if needed
	}
}

func (s *Service) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *Service) Stop(context.Context) error {
	s.cancel()
	// No cleanup needed for metrics
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
func (s *Service) Subscribe(ctx context.Context, ns libshare.Namespace) (<-chan *SubscriptionResponse, error) {
	if s.ctx == nil {
		return nil, fmt.Errorf("service has not been started")
	}

	log.Infow("subscribing for blobs",
		"namespaces", ns.String(),
	)
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
					blobs, err = s.getAll(ctx, header, []libshare.Namespace{ns})
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
func (s *Service) Submit(ctx context.Context, blobs []*Blob, txConfig *SubmitOptions) (_ uint64, err error) {
	ctx, span := tracer.Start(ctx, "blob/submit")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("submitting blobs failed", "err", err,
				"err", err,
			)
		}
	}()

	libBlobs := make([]*libshare.Blob, len(blobs))
	for i := range blobs {
		libBlobs[i] = blobs[i].Blob
	}

	spanCtx := trace.ContextWithSpan(ctx, span)
	resp, err := s.blobSubmitter.SubmitPayForBlob(spanCtx, libBlobs, txConfig)
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
	namespace libshare.Namespace,
	commitment Commitment,
) (blob *Blob, err error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "blob/get")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		s.metrics.observeRetrieval(ctx, time.Since(start), err)
		if err != nil {
			log.Errorw("getting blob",
				"err", err,
				"height", height,
				"namespace", namespace.String(),
				"commitment", commitment.String(),
			)
		}
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)
	log.Infow("getting blob",
		"height", height,
		"namespace", namespace.String(),
		"commitment", commitment.String(),
	)

	sharesParser := &parser{verifyFn: func(blob *Blob) bool {
		return blob.compareCommitments(commitment)
	}}

	blob, _, err = s.retrieve(ctx, height, namespace, sharesParser)
	return blob, err
}

// GetProof returns an NMT inclusion proof for a specified namespace to the respective row roots
// on which the blob spans on at the given height, using the given commitment.
// It employs the same algorithm as service.Get() internally.
func (s *Service) GetProof(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	commitment Commitment,
) (proof *Proof, err error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "blob/get-proof")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		s.metrics.observeProof(ctx, time.Since(start), err)
		if err != nil {
			log.Errorw("getting proof",
				"err", err,
				"height", height,
				"namespace", namespace.String(),
				"commitment", commitment.String(),
			)
		}
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)
	log.Infow("getting proof",
		"height", height,
		"namespace", namespace.String(),
		"commitment", commitment.String(),
	)

	sharesParser := &parser{verifyFn: func(blob *Blob) bool {
		return blob.compareCommitments(commitment)
	}}

	_, proof, err = s.retrieve(ctx, height, namespace, sharesParser)
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
func (s *Service) GetAll(ctx context.Context, height uint64, namespaces []libshare.Namespace) (_ []*Blob, err error) {
	ctx, span := tracer.Start(ctx, "blob/get-all")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting all blobs",
				"err", err,
				"height", height,
				"namespaces", namespaces,
			)
		}
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
	)
	log.Infow("getting all blobs",
		"height", height,
		"namespaces", namespaces,
	)

	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}

	return s.getAll(ctx, header, namespaces)
}

func (s *Service) getAll(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespaces []libshare.Namespace,
) ([]*Blob, error) {
	var (
		span             = trace.SpanFromContext(ctx)
		namespaceStrings = make([]string, len(namespaces))
		resultBlobs      = make([][]*Blob, len(namespaces))
		resultErr        = make([]error, len(namespaces))
		wg               = sync.WaitGroup{}
	)

	for i, namespace := range namespaces {
		wg.Add(1)
		go func(i int, namespace libshare.Namespace) {
			defer wg.Done()
			resultBlobs[i], resultErr[i] = s.getBlobs(ctx, namespace, header)
		}(i, namespace)

		namespaceStrings[i] = namespace.String()
	}
	span.SetAttributes(attribute.StringSlice("namespaces", namespaceStrings))
	wg.Wait()

	blobs := slices.Concat(resultBlobs...)
	err := errors.Join(resultErr...)
	return blobs, err
}

// Included verifies that the blob was included in a specific height.
// To ensure that blob was included in a specific height, we need:
// 1. verify the provided commitment by recomputing it;
// 2. verify the provided Proof against subtree roots that were used in 1.;
func (s *Service) Included(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	proof *Proof,
	commitment Commitment,
) (_ bool, err error) {
	ctx, span := tracer.Start(ctx, "blob/included")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("checking inclusion",
				"err", err,
				"height", height,
				"namespace", namespace.String(),
				"commitment", commitment.String(),
			)
		}
	}()
	span.SetAttributes(
		attribute.Int64("height", int64(height)),
		attribute.String("namespace", namespace.String()),
	)
	log.Infow("included",
		"height", height,
		"namespace", namespace.String(),
		"commitment", commitment.String(),
		"proof-len", proof.Len(),
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
	namespace libshare.Namespace,
	sharesParser *parser,
) (_ *Blob, _ *Proof, err error) {
	log.Infow("retrieving blob",
		"height", height,
		"namespace", namespace.String(),
	)

	span := trace.SpanFromContext(ctx)
	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, nil, err
	}
	span.SetAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots))),
	)

	rowIndex := -1
	for i, row := range header.DAH.RowRoots {
		outside, err := share.IsOutsideRange(namespace, row, row)
		if err != nil {
			return nil, nil, err
		}
		if !outside {
			rowIndex = i
			break
		}
	}
	// collect shares for the requested namespace
	namespacedShares, err := s.shareGetter.GetNamespaceData(ctx, header, namespace)
	if err != nil {
		if errors.Is(err, shwap.ErrNotFound) {
			err = ErrBlobNotFound
		}
		return nil, nil, err
	}

	span.AddEvent("received-shares", trace.WithAttributes(
		attribute.Int("amount", namespacedShares.Length()),
	))

	var (
		appShares = make([]libshare.Share, 0)
		proofs    = make(Proof, 0)
	)

	for _, row := range namespacedShares {
		if len(row.Shares) == 0 {
			// the above condition means that we've faced with an Absence Proof.
			// This Proof proves that the namespace was not found in the DAH, so
			// we can return `ErrBlobNotFound`.
			return nil, nil, ErrBlobNotFound
		}

		appShares = row.Shares
		proofs = append(proofs, row.Proof)
		index := row.Proof.Start()

		for {
			var (
				isComplete bool
				shrs       []libshare.Share
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
		if !sh.IsPadding() {
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
	namespace libshare.Namespace,
	header *header.ExtendedHeader,
) (_ []*Blob, err error) {
	ctx, span := tracer.Start(ctx, "blob/get-blobs-namespace")
	defer utils.SetStatusAndEnd(span, err)
	span.SetAttributes(attribute.String("namespace", namespace.String()))
	log.Debugw("retrieving all blobs from", "namespace", namespace.String(), "height", header.Height())

	blobs := make([]*Blob, 0)
	verifyFn := func(blob *Blob) bool {
		blobs = append(blobs, blob)
		return false
	}
	sharesParser := &parser{verifyFn: verifyFn}

	_, _, err = s.retrieve(ctx, header.Height(), namespace, sharesParser)
	if err != nil && !errors.Is(err, ErrBlobNotFound) {
		log.Errorf("retrieving blobs for the namespace (%s): %v", namespace.String(), err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving blobs for the namespace")
		return nil, err
	}

	log.Infow("retrieved blobs", "namespace", namespace.String(), "height", header.Height(), "total", len(blobs))
	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("total", len(blobs)))
	return blobs, nil
}

func (s *Service) GetCommitmentProof(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	shareCommitment []byte,
) (_ *CommitmentProof, err error) {
	ctx, span := tracer.Start(ctx, "blob/get-commitment-proof")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting commitment proof",
				"err", err,
				"height", height,
				"namespace", namespace.String(),
				"commitment", hex.EncodeToString(shareCommitment),
			)
		}
	}()

	if height == 0 {
		err = fmt.Errorf("height cannot be equal to 0")
		return nil, err
	}

	log.Infow("getting commitment proof",
		"height", height,
		"commitment", hex.EncodeToString(shareCommitment),
		"namespace", namespace,
	)

	// get the blob to compute the subtree roots
	log.Debugw(
		"getting blob",
		"height", height,
		"commitment", hex.EncodeToString(shareCommitment),
		"namespace", namespace,
	)
	blb, err := s.Get(ctx, height, namespace, shareCommitment)
	if err != nil {
		return nil, err
	}

	log.Debugw(
		"converting the blob to shares",
		"height", height,
		"commitment", hex.EncodeToString(shareCommitment),
		"namespace", namespace,
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
		"height", height,
	)
	header, err := s.headerGetter(ctx, height)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.Int64("eds-size", int64(len(header.DAH.RowRoots))),
	)

	log.Debugw("getting eds",
		"height", height,
	)
	eds, err := s.shareGetter.GetEDS(ctx, header)
	if err != nil {
		return nil, err
	}

	return ProveCommitment(eds, namespace, blobShares)
}

func ProveCommitment(
	eds *rsmt2d.ExtendedDataSquare,
	namespace libshare.Namespace,
	blobShares []libshare.Share,
) (_ *CommitmentProof, err error) {
	_, span := tracer.Start(context.Background(), "blob/prove-commitment-proof")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("proving commitment proof",
				"err", err,
				"namespace", namespace.String(),
			)
		}
	}()

	// find the blob shares in the EDS
	blobSharesStartIndex := -1
	for index, share := range eds.FlattenedODS() {
		if bytes.Equal(share, blobShares[0].ToBytes()) {
			blobSharesStartIndex = index
		}
	}
	if blobSharesStartIndex < 0 {
		return nil, fmt.Errorf("couldn't find the blob shares in the ODS")
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
		namespace,
		libshare.NewRange(blobSharesStartIndex, blobSharesStartIndex+len(blobShares)),
	)
	if err != nil {
		return nil, err
	}

	// convert the shares to row root proofs to nmt proofs
	nmtProofs := make([]*nmt.Proof, 0)
	for _, proof := range sharesProof.ShareProofs {
		nmtProof := nmt.NewInclusionProof(
			int(proof.Start),
			int(proof.End),
			proof.Nodes,
			true,
		)
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
			inclusion.SubTreeWidth(len(blobShares), subtreeRootThreshold),
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
		RowProof:          *sharesProof.RowProof,
		NamespaceVersion:  namespace.Version(),
	}
	return &commitmentProof, nil
}

// computeSubtreeRoots takes a set of shares and ranges and returns the corresponding subtree roots.
// the offset is the number of shares that are before the subtree roots we're calculating.
func computeSubtreeRoots(shares []libshare.Share, ranges []nmt.LeafRange, offset int) ([][]byte, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("cannot compute subtree roots for an empty shares list")
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("cannot compute subtree roots for an empty ranges list")
	}
	if offset < 0 {
		return nil, fmt.Errorf("the offset %d cannot be strictly negative", offset)
	}

	// create a tree containing the shares to generate their subtree roots
	tree := nmt.New(
		appconsts.NewBaseHashFunc(),
		nmt.IgnoreMaxNamespace(true),
		nmt.NamespaceIDSize(libshare.NamespaceSize),
	)
	for _, sh := range shares {
		leafData := make([]byte, 0)
		leafData = append(append(leafData, sh.Namespace().Bytes()...), sh.ToBytes()...)
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
