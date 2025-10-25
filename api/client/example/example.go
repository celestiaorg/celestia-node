package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/api/client"
	"github.com/celestiaorg/celestia-node/blob"
)

func main() {
	// Initialize keyring with new key
	keyname := "my_key"
	kr, err := client.KeyringWithNewKey(client.KeyringConfig{
		KeyName:     keyname,
		BackendName: keyring.BackendTest,
	}, "./path_to_keys")
	if err != nil {
		fmt.Println("failed to create keyring:", err)
		return
	}

	// Configure client with multiple endpoints for failover
	cfg := client.Config{
		ReadConfig: client.ReadConfig{
			BridgeDAAddr: "http://localhost:26658",
			AdditionalBridgeDAAddrs: []string{
				"http://backup-bridge-1:26658",
				"http://backup-bridge-2:26658",
			},
			DAAuthToken: "token",
		},
		SubmitConfig: client.SubmitConfig{
			DefaultKeyName: keyname,
			Network:        "mocha-4",
			CoreGRPCConfig: client.CoreGRPCConfig{
				Addr: "celestia-testnet-consensus.itrocket.net:9090",
				AdditionalCoreGRPCConfigs: []client.CoreGRPCConfig{
					{
						Addr:       "backup-consensus-1.itrocket.net:9090",
						TLSEnabled: false,
						AuthToken:  "",
					},
					{
						Addr:       "backup-consensus-2.itrocket.net:9090",
						TLSEnabled: false,
						AuthToken:  "",
					},
				},
				TLSEnabled: false,
				AuthToken:  "",
			},
		},
	}

	// Create multi-endpoint client with failover capabilities
	ctx := context.Background()
	celestiaClient, err := client.NewMultiEndpoint(ctx, cfg, kr)
	if err != nil {
		fmt.Println("failed to create multi-endpoint client:", err)
		return
	}
	defer celestiaClient.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	// Submit a blob
	namespace := libshare.MustNewV0Namespace([]byte("example"))
	b, err := blob.NewBlob(libshare.ShareVersionZero, namespace, []byte("data"), nil)
	if err != nil {
		fmt.Println("failed to create blob:", err)
		return
	}
	height, err := celestiaClient.Blob.Submit(ctx, []*blob.Blob{b}, nil)
	if err != nil {
		fmt.Println("failed to submit blob:", err)
		return
	}
	fmt.Println("submitted blob", height)

	// Retrieve a blob
	retrievedBlob, err := celestiaClient.Blob.Get(ctx, height, namespace, b.Commitment)
	if err != nil {
		fmt.Println("failed to retrieve blob:", err)
		return
	}
	fmt.Println("retrieved blob", string(retrievedBlob.Data()))

	// Demonstrate access to multiple connections
	fmt.Printf("Available read clients: %d\n", len(celestiaClient.GetReadClients()))
	fmt.Printf("Available gRPC connections: %d\n", len(celestiaClient.GetGRPCConnections()))
}
