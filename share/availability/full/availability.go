package full

import (
	"context"
	"errors"

	"github.com/ipfs/go-blockservice"
	ipldFormat "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/eds"
	shrex "github.com/celestiaorg/celestia-node/share/eds/p2p"

	"github.com/celestiaorg/rsmt2d"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	ipldRetriever *eds.Retriever
	disc          *discovery.Discovery
	client        *shrex.Client
	store         *eds.Store

	cancel context.CancelFunc
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	bServ blockservice.BlockService,
	disc *discovery.Discovery,
	client *shrex.Client,
	store *eds.Store,
) *ShareAvailability {
	return &ShareAvailability{
		ipldRetriever: eds.NewRetriever(bServ),
		disc:          disc,
		client:        client,
		store:         store,
	}
}

func (fa *ShareAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	fa.cancel = cancel

	go fa.disc.Advertise(ctx)
	go fa.disc.EnsurePeers(ctx)
	return nil
}

func (fa *ShareAvailability) Stop(context.Context) error {
	fa.cancel()
	return nil
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, root *share.Root, peers ...peer.ID) error {
	ctx, cancel := context.WithTimeout(ctx, share.AvailabilityTimeout)
	defer cancel()
	// we assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	err := root.ValidateBasic()
	if err != nil {
		log.Errorw("availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}

	// don't retrieve from network if we already have it locally
	if ok, _ := fa.store.Has(ctx, root.Hash()); ok {
		return nil
	}

	// if no peers were given, use the discovery service
	if len(peers) == 0 {
		peers, err = fa.disc.Peers(ctx)
		if err != nil {
			log.Errorw("no peers were given and the discovery service failed", "err", err)
		}
	}

	var retrievedEDS *rsmt2d.ExtendedDataSquare
	// happy path: try to retrieve the EDS using the ShrEx/EDS protocol within p2p.BlockTime
	retrievedEDS = fa.retrieve(ctx, root, peers...)
	if retrievedEDS != nil {
		fa.storeEDS(ctx, root, retrievedEDS)
		return nil
	}

	// fallback path: try to retrieve the EDS using IPLD network
	retrievedEDS, err = fa.retrieveOverIPLD(ctx, root)
	if err != nil {
		return err
	}
	fa.storeEDS(ctx, root, retrievedEDS)
	return nil
}

// retrieve attempts to retrieve the EDS using the ShrEx/EDS protocol within p2p.BlockTime.
func (fa *ShareAvailability) retrieve(
	ctx context.Context,
	root *share.Root,
	peers ...peer.ID,
) *rsmt2d.ExtendedDataSquare {
	reqCtx, cancel := context.WithTimeout(ctx, p2p.BlockTime)
	defer cancel()
	retrievedEDS, err := fa.client.RequestEDS(reqCtx, root.Hash(), peers)
	if err != nil {
		// errors are logged but not returned to pass retrieval to the fallback method.
		log.Errorw("availability validation failed over ShrEx/EDS", "root", root.String(), "err", err)
	}

	return retrievedEDS
}

// retrieveOverIPLD attempts to retrieve the EDS using the IPLD network. It is a fallback method for
// ShrEx/EDS retrieval failures.
func (fa *ShareAvailability) retrieveOverIPLD(
	ctx context.Context,
	root *share.Root,
) (*rsmt2d.ExtendedDataSquare, error) {
	retrievedEDS, err := fa.ipldRetriever.Retrieve(ctx, root)
	if err != nil {
		log.Errorw("availability validation failed over IPLD", "root", root.String(), "err", err)
		if ipldFormat.IsNotFound(err) || errors.Is(err, context.DeadlineExceeded) {
			return nil, share.ErrNotAvailable
		}

		return nil, err
	}

	return retrievedEDS, nil
}

// storeEDS stores a retrieved EDS in the local EDS dagstore.
func (fa *ShareAvailability) storeEDS(ctx context.Context, root *share.Root, eds *rsmt2d.ExtendedDataSquare) {
	err := fa.store.Put(ctx, root.Hash(), eds)
	if err != nil {
		// errors are logged but not returned, because a failure to store the retrieved EDS doesn't
		// mean the shares were not available.
		log.Errorw("failed to store retrieved EDS", "err", err)
	}
}

func (fa *ShareAvailability) ProbabilityOfAvailability(context.Context) float64 {
	return 1
}
