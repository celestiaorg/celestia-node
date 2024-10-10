//go:build da || integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
)

var namespace, _ = share.NewBlobNamespaceV0([]byte("namespace"))

type daModuleCorpus struct {
	Blobs [][]byte `json:"blobs"`
}

func FuzzDaModule(f *testing.F) {
	// 1. Retrieve the fuzz corpra.
	corpraPath := filepath.Join("testdata", "bytes-submit", "*.bytes")
	globbed, err := filepath.Glob(corpraPath)
	if err != nil {
		f.Fatal(err)
	}

	blobs := [][]byte{}
	for _, path := range globbed {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		blobs = append(blobs, data)
	}

	jsonBlob, err := json.Marshal(&daModuleCorpus{Blobs: blobs})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(jsonBlob)

	dmc := new(daModuleCorpus)
	if err := json.Unmarshal(jsonBlob, dmc); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

        t := new(testing.T)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(10*time.Second))

	bridge := sw.NewBridgeNode()
	if err := bridge.Start(ctx); err != nil {
		return
	}

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	if err != nil {
		return
	}

	fullCfg := sw.DefaultTestConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	fullNode := sw.NewNodeWithConfig(node.Full, fullCfg)
	require.NoError(f, fullNode.Start(ctx))

	addrsFull, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(fullNode.Host))
	if err != nil {
		return
	}

	lightCfg := sw.DefaultTestConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrsFull[0].String())
	lightNode := sw.NewNodeWithConfig(node.Light, lightCfg)
	if err := lightNode.Start(ctx); err != nil {
		return
	}

	fullClient := getAdminClient(ctx, fullNode, t)
	lightClient := getAdminClient(ctx, lightNode, t)

	// 2. Now run the fuzzers.
	f.Fuzz(func(t *testing.T, jsonBlob []byte) {
		daBlobs := dmc.Blobs
		ids, err := fullClient.DA.Submit(ctx, daBlobs, -1, namespace)
		if err != nil {
			return
		}

		tests := []struct {
			name string
			doFn func(t *testing.T)
		}{
			{
				name: "MaxBlobSize",
				doFn: func(t *testing.T) {
					_, _ = fullClient.DA.MaxBlobSize(ctx)
				},
			},
			{
				name: "GetProofs + Validate",
				doFn: func(t *testing.T) {
					h, _ := da.SplitID(ids[0])
					lightClient.Header.WaitForHeight(ctx, h)
					proofs, err := lightClient.DA.GetProofs(ctx, ids, namespace)
					if err != nil {
						return
					}
					if len(proofs) == 0 {
						return
					}
					_, _ = fullClient.DA.Validate(ctx, ids, proofs, namespace)
				},
			},
			{
				name: "GetIDs",
				doFn: func(t *testing.T) {
					height, _ := da.SplitID(ids[0])
					_, err := fullClient.DA.GetIDs(ctx, height, namespace)
					if err != nil {
						return
					}
					_, _ = lightClient.Header.GetByHeight(ctx, height)
				},
			},
			{
				name: "Get",
				doFn: func(t *testing.T) {
					h, _ := da.SplitID(ids[0])
					lightClient.Header.WaitForHeight(ctx, h)
					fetched, err := lightClient.DA.Get(ctx, ids, namespace)
					if err != nil {
						return
					}
					if g, w := len(fetched), len(ids); g != w {
						t.Fatalf("len(fetched)=%d != len(ids)=%d", g, w)
					}
					for i := range fetched {
						if !bytes.Equal(fetched[i], daBlobs[i]) {
							t.Errorf("!bytes.Equal(fetched[%d], daBlbos[%d]\n\tFetched: % x\n\tDaBlobs: % x\n", i, i, fetched[i], daBlobs[i])
						}
					}
				},
			},
			{
				name: "Commit",
				doFn: func(t *testing.T) {
					fetched, err := fullClient.DA.Commit(ctx, daBlobs, namespace)
					require.NoError(t, err)
					require.Len(t, fetched, len(ids))
					for i := range fetched {
						_, commitment := da.SplitID(ids[i])
						require.EqualValues(t, fetched[i], commitment)
					}
				},
			},
			{
				name: "SubmitWithOptions - valid",
				doFn: func(t *testing.T) {
					ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte(`{"key_name": "validator"}`))
					require.NoError(t, err)
					require.NotEmpty(t, ids)
				},
			},
			{
				name: "SubmitWithOptions - invalid JSON",
				doFn: func(t *testing.T) {
					ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte("not JSON"))
					require.Error(t, err)
					require.Nil(t, ids)
				},
			},
			{
				name: "SubmitWithOptions - invalid key name",
				doFn: func(t *testing.T) {
					ids, err := fullClient.DA.SubmitWithOptions(ctx, daBlobs, -1, namespace, []byte(`{"key_name": "invalid"}`))
					require.Error(t, err)
					require.Nil(t, ids)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.doFn(t)
			})
		}
	})
}
