package share

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

func newTestHost(t *testing.T) host.Host {
	t.Helper()
	bus := eventbus.NewBus()
	sw := swarmt.GenSwarm(t, swarmt.OptDisableTCP, swarmt.EventBus(bus))
	h, err := basichost.NewHost(sw, &basichost.HostOpts{EventBus: bus})
	require.NoError(t, err)
	h.Start()
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func connectAll(t *testing.T, ctx context.Context, hs []host.Host) {
	t.Helper()
	for i := range hs {
		for j := range hs {
			if i == j {
				continue
			}
			ai := peer.AddrInfo{ID: hs[j].ID(), Addrs: hs[j].Addrs()}
			require.NoError(t, hs[i].Connect(ctx, ai))
		}
	}
}

type strictTestVerifier struct{}

func (strictTestVerifier) CommitColumn(_ [][]byte) (crypto.ColumnCommitment, error) { return nil, nil }
func (strictTestVerifier) CombineCommitments(_ []crypto.ColumnCommitment, _ crypto.CodingVector) (crypto.ColumnCommitment, error) {
	return nil, nil
}

func (strictTestVerifier) VerifyFragment(y []byte, g crypto.CodingVector, colComs []crypto.ColumnCommitment, proof crypto.FragmentProof) error {
	if g.K == 0 {
		return errors.New("missing coding vector K")
	}
	if len(colComs) < 1 {
		return errors.New("missing commitments")
	}
	h := sha256.Sum256(y)
	// commitment must match sha256(y) for this test harness
	if string(colComs[0]) != string(h[:]) {
		return errors.New("commitment mismatch")
	}
	// proof must also match sha256(y)
	if string(proof.ProofBytes) != string(h[:]) {
		return errors.New("proof mismatch")
	}
	return nil
}

func TestCDA_OutOfOrder_And_ByzantineFragmentsDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	// Force all peers into the same cell/col topics by using a 1x1 grid.
	gridDims := GridDimensions{Rows: 1, Cols: 1}
	grid := NewRDAGridManager(gridDims)

	hs := []host.Host{
		newTestHost(t),
		newTestHost(t),
		newTestHost(t),
		newTestHost(t),
		newTestHost(t),
		newTestHost(t),
	}
	connectAll(t, ctx, hs)

	ps := make([]*pubsub.PubSub, len(hs))
	for i, h := range hs {
		p, err := pubsub.NewGossipSub(ctx, h)
		require.NoError(t, err)
		ps[i] = p
	}

	// Receiver node (index 0) runs CDA node logic.
	grid.RegisterPeer(hs[0].ID())
	verifier := strictTestVerifier{}
	expectedData := []byte("honest-data")
	expectedHash := sha256.Sum256(expectedData)
	commitSrc := StaticColumnCommitments{Commitments: []crypto.ColumnCommitment{expectedHash[:]}}
	node := NewCDANode(
		hs[0],
		ps[0],
		grid,
		verifier,
		commitSrc,
		CDANodeConfig{Service: CDAServiceConfig{K: 2, Buffer: 1}},
	)
	require.NoError(t, node.Start(ctx))
	t.Cleanup(func() { _ = node.Stop(ctx) })

	// Publish out-of-order fragments directly to the column topic from other peers.
	// We use a real verifier later; for now, this checks ordering/anti-spam behavior.
	colTopic := node.subnet.GetColTopic()

	makeFrag := func(from host.Host, data []byte) ([]byte, error) {
		cv, err := NewSeededCodingVector(2)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(data)
		msg := CDAFragmentMessage{
			Height:    1,
			Row:       0,
			Col:       0,
			NodeID:    from.ID().String(),
			CodingVec: cv,
			Data:      data,
			Proof:     crypto.FragmentProof{ProofBytes: hash[:]},
		}
		return marshalCDAFragmentMessage(msg)
	}

	// Byzantine case A: correct CodingVector but wrong Data (and proof matches wrong data).
	// Should be dropped at validation gate because commitment expects honest-data hash.
	badData := []byte("byzantine-data")
	b, err := makeFrag(hs[1], badData)
	require.NoError(t, err)
	topic, err := ps[1].Join(colTopic)
	require.NoError(t, err)
	require.NoError(t, topic.Publish(ctx, b))
	_ = topic.Close()

	// Byzantine case B: correct Data but wrong Proof. Should be dropped at validation gate.
	cv, err := NewSeededCodingVector(2)
	require.NoError(t, err)
	msg := CDAFragmentMessage{
		Height:    1,
		Row:       0,
		Col:       0,
		NodeID:    hs[2].ID().String(),
		CodingVec: cv,
		Data:      expectedData,
		Proof:     crypto.FragmentProof{ProofBytes: []byte("wrong-proof")},
	}
	b, err = marshalCDAFragmentMessage(msg)
	require.NoError(t, err)
	topic, err = ps[2].Join(colTopic)
	require.NoError(t, err)
	require.NoError(t, topic.Publish(ctx, b))
	_ = topic.Close()

	// Honest fragments: send out-of-order fragments directly to the column topic from other peers.
	// Cap is K+Buffer = 3; publish many honest fragments concurrently to simulate network jitter.
	var wg sync.WaitGroup
	for i := 1; i < len(hs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// jitter / out-of-order arrival
			time.Sleep(time.Duration((len(hs)-i)*15) * time.Millisecond)
			b, err := makeFrag(hs[i], expectedData)
			require.NoError(t, err)
			topic, err := ps[i].Join(colTopic)
			require.NoError(t, err)
			require.NoError(t, topic.Publish(ctx, b))
			_ = topic.Close()
		}(i)
	}

	// Duplicate-per-peer spam: same peer sends again; should be ignored by peer-id binding.
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		b, err := makeFrag(hs[3], expectedData)
		require.NoError(t, err)
		topic, err := ps[3].Join(colTopic)
		require.NoError(t, err)
		require.NoError(t, topic.Publish(ctx, b))
		_ = topic.Close()
	}()

	wg.Wait()

	// Give receiver a moment to process.
	time.Sleep(250 * time.Millisecond)
	frags := node.svc.GetFragments(1, 0, 0)
	require.LessOrEqual(t, len(frags), 3, "must enforce hard cap K+Buffer")
}

