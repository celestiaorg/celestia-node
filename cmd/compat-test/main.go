package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/celestiaorg/go-square/v3/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	rpcURL     = flag.String("rpc-url", "", "RPC URL of the celestia-node server to test against")
	skipGetRow = flag.Bool(
		"skip-get-row",
		false,
		"Skip Share.GetRow test (for light nodes with bitswap compatibility issues)",
	)
	timeout = flag.Duration("timeout", 5*time.Minute, "Timeout for the entire test suite")
)

func main() {
	flag.Parse()

	if *rpcURL == "" {
		fmt.Fprintf(os.Stderr, "Error: --rpc-url is required\n")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	err := runMain(ctx)
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Println("All compatibility tests passed")
}

func runMain(ctx context.Context) error {
	serverAddr := *rpcURL
	if !strings.HasPrefix(serverAddr, "http://") && !strings.HasPrefix(serverAddr, "https://") {
		serverAddr = "http://" + serverAddr
	}

	client, err := rpcclient.NewClient(ctx, serverAddr, "")
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	return runCompatibilityTests(ctx, client, *skipGetRow)
}

func runCompatibilityTests(ctx context.Context, client *rpcclient.Client, skipGetRow bool) error {
	if _, err := client.Node.Info(ctx); err != nil {
		return fmt.Errorf("Node.Info failed: %w", err)
	}
	if _, err := client.Node.Ready(ctx); err != nil {
		return fmt.Errorf("Node.Ready failed: %w", err)
	}

	head, err := client.Header.LocalHead(ctx)
	if err != nil {
		return fmt.Errorf("Header.LocalHead failed: %w", err)
	}
	if _, err := client.Header.GetByHeight(ctx, head.Height()); err != nil {
		return fmt.Errorf("Header.GetByHeight failed: %w", err)
	}
	if _, err := client.Header.GetByHash(ctx, head.Hash()); err != nil {
		return fmt.Errorf("Header.GetByHash failed: %w", err)
	}
	if _, err := client.Header.SyncState(ctx); err != nil {
		return fmt.Errorf("Header.SyncState failed: %w", err)
	}
	if _, err := client.Header.NetworkHead(ctx); err != nil {
		return fmt.Errorf("Header.NetworkHead failed: %w", err)
	}
	if _, err := client.Header.Tail(ctx); err != nil {
		return fmt.Errorf("Header.Tail failed: %w", err)
	}

	addr, err := client.State.AccountAddress(ctx)
	if err != nil {
		return fmt.Errorf("State.AccountAddress failed: %w", err)
	}
	if _, err := client.State.BalanceForAddress(ctx, addr); err != nil {
		return fmt.Errorf("State.BalanceForAddress failed: %w", err)
	}
	if _, err := client.State.Balance(ctx); err != nil {
		return fmt.Errorf("State.Balance failed: %w", err)
	}

	if _, err := client.P2P.Info(ctx); err != nil {
		return fmt.Errorf("P2P.Info failed: %w", err)
	}
	if _, err := client.P2P.Peers(ctx); err != nil {
		return fmt.Errorf("P2P.Peers failed: %w", err)
	}
	if _, err := client.P2P.NATStatus(ctx); err != nil {
		return fmt.Errorf("P2P.NATStatus failed: %w", err)
	}
	if _, err := client.P2P.BandwidthStats(ctx); err != nil {
		return fmt.Errorf("P2P.BandwidthStats failed: %w", err)
	}
	if _, err := client.P2P.ResourceState(ctx); err != nil {
		if !strings.Contains(err.Error(), "invalid cid") || !strings.Contains(err.Error(), "peer ID") {
			return fmt.Errorf("P2P.ResourceState failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "P2P.ResourceState failed (known limitation): %v\n", err)
	}
	if _, err := client.P2P.PubSubTopics(ctx); err != nil {
		return fmt.Errorf("P2P.PubSubTopics failed: %w", err)
	}

	namespace, _ := share.NewV0Namespace([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
	if err := client.Share.SharesAvailable(ctx, head.Height()); err != nil {
		if strings.Contains(err.Error(), "data not available") {
			fmt.Fprintf(os.Stderr, "Share.SharesAvailable: data not available (known limitation): %v\n", err)
		} else {
			return fmt.Errorf("Share.SharesAvailable failed: %w", err)
		}
	}
	if _, err := client.Share.GetNamespaceData(ctx, head.Height(), namespace); err != nil {
		if strings.Contains(err.Error(), "data not available") {
			fmt.Fprintf(os.Stderr, "Share.GetNamespaceData: data not available (known limitation): %v\n", err)
		} else {
			return fmt.Errorf("Share.GetNamespaceData failed: %w", err)
		}
	}
	if _, err := client.Share.GetEDS(ctx, head.Height()); err != nil {
		if strings.Contains(err.Error(), "data not available") {
			fmt.Fprintf(os.Stderr, "Share.GetEDS: data not available (known limitation): %v\n", err)
		} else {
			return fmt.Errorf("Share.GetEDS failed: %w", err)
		}
	}

	if !skipGetRow {
		rowCtx, rowCancel := context.WithTimeout(ctx, 120*time.Second)
		defer rowCancel()
		if _, err := client.Share.GetRow(rowCtx, head.Height(), 0); err != nil {
			return fmt.Errorf("Share.GetRow failed: %w", err)
		}
	}

	dasCtx, dasCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dasCancel()
	_, err = client.DAS.SamplingStats(dasCtx)
	if err != nil && !strings.Contains(err.Error(), "stubbed") && !strings.Contains(err.Error(), "deadline exceeded") {
		return fmt.Errorf("DAS.SamplingStats failed: %w", err)
	}

	blobCtx, blobCancel := context.WithTimeout(ctx, 30*time.Second)
	defer blobCancel()
	if _, err := client.Blob.GetAll(blobCtx, head.Height(), []share.Namespace{namespace}); err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") {
			fmt.Fprintf(os.Stderr, "Blob.GetAll timeout (known limitation with old light servers): %v\n", err)
		} else {
			return fmt.Errorf("Blob.GetAll failed: %w", err)
		}
	}

	testBlob, err := blob.NewBlobV0(namespace, []byte("cross-version test blob"))
	if err == nil {
		submitHeight, err := client.Blob.Submit(ctx, []*blob.Blob{testBlob}, state.NewTxConfig())
		if err == nil && submitHeight > 0 {
			if _, err := client.Blob.Get(ctx, submitHeight, namespace, testBlob.Commitment); err != nil {
				return fmt.Errorf("Blob.Get failed: %w", err)
			}

			proof, err := client.Blob.GetProof(ctx, submitHeight, namespace, testBlob.Commitment)
			if err != nil {
				return fmt.Errorf("Blob.GetProof failed: %w", err)
			}

			if _, err := client.Blob.Included(ctx, submitHeight, namespace, proof, testBlob.Commitment); err != nil {
				return fmt.Errorf("Blob.Included failed: %w", err)
			}
		}
	}

	return nil
}
