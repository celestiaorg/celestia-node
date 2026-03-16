package share

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

// ColumnCommitmentProvider provides KZG column commitments for a given block height.
// CDA uses this to validate fragments received over the network.
type ColumnCommitmentProvider interface {
	// ColumnCommitments returns the set of commitments Com(x_i) needed to validate fragments.
	// The exact meaning (commitments per share, per column, per piece) will be refined as
	// the header format is updated; for now, this is a minimal hook.
	ColumnCommitments(ctx context.Context, height uint64, col uint16) ([]crypto.ColumnCommitment, error)
}

// CDANode wires CDA pubsub topics to the CDAService storage-policy core.
// It is an incremental scaffold; later iterations will add full Stage 2/3 flows
// (STORE to cell, RLNC transform, and column gossip) and replace JSON messages with Protobuf.
type CDANode struct {
	host   host.Host
	subnet *CDASubnetManager
	svc    *CDAService

	verifier  crypto.HomomorphicKZG
	commitSrc ColumnCommitmentProvider

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type CDANodeConfig struct {
	Service CDAServiceConfig
}

func NewCDANode(
	h host.Host,
	ps *pubsub.PubSub,
	grid *RDAGridManager,
	verifier crypto.HomomorphicKZG,
	commitSrc ColumnCommitmentProvider,
	cfg CDANodeConfig,
) *CDANode {
	ctx, cancel := context.WithCancel(context.Background())

	subnet := NewCDASubnetManager(ps, grid, h.ID())
	svc := NewCDAService(cfg.Service)

	return &CDANode{
		host:      h,
		subnet:    subnet,
		svc:       svc,
		verifier:  verifier,
		commitSrc: commitSrc,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (n *CDANode) Start(ctx context.Context) error {
	if err := n.subnet.Start(ctx); err != nil {
		return err
	}

	n.wg.Add(2)
	go n.runCellReceiver()
	go n.runColReceiver()
	return nil
}

func (n *CDANode) Stop(ctx context.Context) error {
	n.cancel()
	_ = n.subnet.Stop(ctx)
	n.wg.Wait()
	return nil
}

// Service exposes the underlying CDAService (used by CDA availability / DAS integration).
func (n *CDANode) Service() *CDAService {
	return n.svc
}

// Verifier exposes the configured homomorphic KZG verifier.
func (n *CDANode) Verifier() crypto.HomomorphicKZG {
	return n.verifier
}

// CommitmentSource exposes the commitment provider used by this node.
func (n *CDANode) CommitmentSource() ColumnCommitmentProvider {
	return n.commitSrc
}

func (n *CDANode) runCellReceiver() {
	defer n.wg.Done()
	ch := n.subnet.ReceiveFromCell(n.ctx)
	for {
		select {
		case <-n.ctx.Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			typ, store, frag, err := unmarshalCDAMessage(b)
			if err != nil {
				continue
			}
			switch typ {
			case cdaMsgTypeStoreRawShare:
				if store != nil {
					_ = n.handleStore(*store)
				}
			case cdaMsgTypeFragment:
				// Ignore: fragments belong to column receiver.
				_ = frag
			}
		}
	}
}

func (n *CDANode) handleStore(m CDAStoreMessage) error {
	// Minimal Stage 2/3 scaffold:
	// - Create a seeded coding vector of length K
	// - Publish a fragment message to the column topic
	//
	// RLNC piece-splitting + KZG proof generation will be implemented once the
	// field arithmetic and KZG prover are integrated.
	cv, err := NewSeededCodingVector(n.svc.cfg.K)
	if err != nil {
		return err
	}
	// For now, carry the raw share bytes as fragment payload.
	frag := CDAFragmentMessage{
		Height:    m.Height,
		Row:       m.Row,
		Col:       m.Col,
		NodeID:    n.host.ID().String(),
		CodingVec: cv,
		Data:      m.Share,
		Proof:     crypto.FragmentProof{},
	}
	b, err := marshalCDAFragmentMessage(frag)
	if err != nil {
		return err
	}
	return n.subnet.PublishToCol(n.ctx, b)
}

func (n *CDANode) runColReceiver() {
	defer n.wg.Done()
	ch := n.subnet.ReceiveFromCol(n.ctx)
	for {
		select {
		case <-n.ctx.Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			typ, store, frag, err := unmarshalCDAMessage(b)
			if err != nil {
				continue
			}
			switch typ {
			case cdaMsgTypeFragment:
				if frag == nil {
					continue
				}
				if err := n.handleFragment(*frag); err != nil {
					continue
				}
			case cdaMsgTypeStoreRawShare:
				// Ignore: raw shares belong to cell receiver.
				_ = store
			}
		}
	}
}

func (n *CDANode) handleFragment(m CDAFragmentMessage) error {
	pid, err := peer.Decode(m.NodeID)
	if err != nil {
		return err
	}
	// Ignore our own fragments to avoid redundant work.
	if pid == n.host.ID() {
		return nil
	}

	f := Fragment{
		ID: FragmentID{
			Height: m.Height,
			Row:    m.Row,
			Col:    m.Col,
			NodeID: pid,
		},
		CodingVec: m.CodingVec,
		Data:      m.Data,
		Proof:     m.Proof,
	}

	if n.commitSrc == nil {
		return fmt.Errorf("cda: missing commitment provider")
	}
	if n.verifier == nil {
		return fmt.Errorf("cda: missing kzg verifier")
	}

	colComs, err := n.commitSrc.ColumnCommitments(n.ctx, m.Height, m.Col)
	if err != nil {
		return err
	}

	// Enforce: cryptographic validity gate before storage-policy.
	_, err = n.svc.ValidateAndStoreFragment(n.verifier, colComs, f)
	// Note: we intentionally do not re-publish fragments here. GossipSub already
	// propagates messages. This also satisfies the anti-spam requirement that
	// late-arriving (even valid) fragments beyond the hard cap should not be
	// re-propagated.
	return err
}

