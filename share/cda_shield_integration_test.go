package share

import (
	"context"
	"math/big"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/require"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

func newShieldTestHost(t *testing.T) host.Host {
	t.Helper()
	bus := eventbus.NewBus()
	sw := swarmt.GenSwarm(t, swarmt.OptDisableTCP, swarmt.EventBus(bus))
	h, err := basichost.NewHost(sw, &basichost.HostOpts{EventBus: bus})
	require.NoError(t, err)
	h.Start()
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func connectPair(t *testing.T, ctx context.Context, a, b host.Host) {
	t.Helper()
	ai := peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}
	require.NoError(t, a.Connect(ctx, ai))
	bi := peer.AddrInfo{ID: a.ID(), Addrs: a.Addrs()}
	require.NoError(t, b.Connect(ctx, bi))
}

func TestCDA_RealShield_DropsWrongProof(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// 1x1 grid so everyone uses the same topics.
	grid := NewRDAGridManager(GridDimensions{Rows: 1, Cols: 1})

	recvHost := newShieldTestHost(t)
	sendHost := newShieldTestHost(t)
	connectPair(t, ctx, recvHost, sendHost)

	recvPS, err := pubsub.NewGossipSub(ctx, recvHost)
	require.NoError(t, err)
	sendPS, err := pubsub.NewGossipSub(ctx, sendHost)
	require.NoError(t, err)

	grid.RegisterPeer(recvHost.ID())

	verifier := crypto.NewBLS12381HomomorphicKZG()

	// Build two column commitments com0, com1 as scalar multiples of generator.
	_, _, g1Aff, _ := bls12381.Generators()
	var x0, x1 blsfr.Element
	x0.SetUint64(11)
	x1.SetUint64(29)
	var p0, p1 bls12381.G1Affine
	p0.ScalarMultiplication(&g1Aff, x0.BigInt(new(big.Int)))
	p1.ScalarMultiplication(&g1Aff, x1.BigInt(new(big.Int)))
	b0 := p0.Bytes()
	b1 := p1.Bytes()
	com0 := crypto.ColumnCommitment(b0[:])
	com1 := crypto.ColumnCommitment(b1[:])

	commitSrc := StaticColumnCommitments{Commitments: []crypto.ColumnCommitment{com0, com1}}

	node := NewCDANode(
		recvHost,
		recvPS,
		grid,
		verifier,
		commitSrc,
		CDANodeConfig{Service: CDAServiceConfig{K: 2, Buffer: 0}},
	)
	require.NoError(t, node.Start(ctx))
	t.Cleanup(func() { _ = node.Stop(ctx) })

	colTopic := node.subnet.GetColTopic()

	// coding vector g = [2, 5]
	var g0, g1 blsfr.Element
	g0.SetUint64(2)
	g1.SetUint64(5)
	gb0 := g0.Bytes()
	gb1 := g1.Bytes()
	cv := crypto.CodingVector{
		K: 2,
		Coeffs: []crypto.FieldElement{
			{Bytes: gb0[:]},
			{Bytes: gb1[:]},
		},
	}

	// Proof is Com(y) = sum g_i * Com(x_i)
	proofBytes, err := verifier.CombineCommitments([]crypto.ColumnCommitment{com0, com1}, cv)
	require.NoError(t, err)

	makeMsg := func(proof []byte) ([]byte, error) {
		return marshalCDAFragmentMessage(CDAFragmentMessage{
			Height:    1,
			Row:       0,
			Col:       0,
			NodeID:    sendHost.ID().String(),
			CodingVec: cv,
			Data:      []byte("payload-bytes-ignored-for-now"),
			Proof:     crypto.FragmentProof{ProofBytes: proof},
		})
	}

	// Publish honest fragment.
	b, err := makeMsg(proofBytes)
	require.NoError(t, err)
	topic, err := sendPS.Join(colTopic)
	require.NoError(t, err)
	require.NoError(t, topic.Publish(ctx, b))
	_ = topic.Close()

	var frags []Fragment
	require.Eventually(t, func() bool {
		frags = node.svc.GetFragments(1, 0, 0)
		return len(frags) == 1
	}, 2*time.Second, 25*time.Millisecond)

	// Publish byzantine fragment: wrong proof.
	badProof := append([]byte{}, proofBytes...)
	badProof[0] ^= 0xFF
	b, err = makeMsg(badProof)
	require.NoError(t, err)
	topic, err = sendPS.Join(colTopic)
	require.NoError(t, err)
	require.NoError(t, topic.Publish(ctx, b))
	_ = topic.Close()

	time.Sleep(200 * time.Millisecond)
	frags = node.svc.GetFragments(1, 0, 0)
	require.Len(t, frags, 1, "wrong proof must be dropped at validation gate")
}

