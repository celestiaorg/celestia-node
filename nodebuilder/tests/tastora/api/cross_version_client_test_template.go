//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/go-square/v3/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	serverAddr := "SERVER_ADDR_PLACEHOLDER"
	if !hasProtocol(serverAddr) {
		serverAddr = "http://" + serverAddr
	}

	client, err := rpcclient.NewClient(ctx, serverAddr, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	var hasErrors bool

	// Node API
	if info, err := client.Node.Info(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Node.Info failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Node.Info: Type=%s, APIVersion=%s\n", info.Type, info.APIVersion)
	}

	if ready, err := client.Node.Ready(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Node.Ready failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Node.Ready: %v\n", ready)
	}

	// Header API
	if head, err := client.Header.LocalHead(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Header.LocalHead failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Header.LocalHead: Height=%d\n", head.Height())
		if _, err := client.Header.GetByHeight(ctx, head.Height()); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Header.GetByHeight failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ Header.GetByHeight: OK\n")
		}
		if _, err := client.Header.GetByHash(ctx, head.Hash()); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Header.GetByHash failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ Header.GetByHash: OK\n")
		}
	}

	if syncState, err := client.Header.SyncState(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Header.SyncState failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Header.SyncState: Syncing=%v\n", syncState.Syncing)
	}

	if networkHead, err := client.Header.NetworkHead(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Header.NetworkHead failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Header.NetworkHead: Height=%d\n", networkHead.Height())
	}

	if tail, err := client.Header.Tail(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Header.Tail failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Header.Tail: Height=%d\n", tail.Height())
	}

	// State API
	if addr, err := client.State.AccountAddress(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ State.AccountAddress failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ State.AccountAddress: %s\n", addr.String())
		if _, err := client.State.BalanceForAddress(ctx, addr); err != nil {
			fmt.Fprintf(os.Stderr, "✗ State.BalanceForAddress failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ State.BalanceForAddress: OK\n")
		}
	}

	if balance, err := client.State.Balance(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ State.Balance failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ State.Balance: %s\n", balance.String())
	}

	// P2P API
	if info, err := client.P2P.Info(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.Info failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.Info: ID=%s\n", info.ID.String())
	}

	if peers, err := client.P2P.Peers(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.Peers failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.Peers: Count=%d\n", len(peers))
	}

	if natStatus, err := client.P2P.NATStatus(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.NATStatus failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.NATStatus: Reachable=%v\n", natStatus.Reachable)
	}

	if bandwidth, err := client.P2P.BandwidthStats(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.BandwidthStats failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.BandwidthStats: TotalIn=%d, TotalOut=%d\n", bandwidth.TotalIn, bandwidth.TotalOut)
	}

	if resourceState, err := client.P2P.ResourceState(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.ResourceState failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.ResourceState: NumStreams=%d\n", resourceState.NumStreams)
	}

	if topics, err := client.P2P.PubSubTopics(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ P2P.PubSubTopics failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ P2P.PubSubTopics: Count=%d\n", len(topics))
	}

	// Share API
	namespace := share.Namespace{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	if available, err := client.Share.SharesAvailable(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Share.SharesAvailable failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ Share.SharesAvailable: %v\n", available)
	}

	if head, err := client.Header.LocalHead(ctx); err == nil {
		if _, err := client.Share.GetNamespaceData(ctx, head, namespace); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Share.GetNamespaceData failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ Share.GetNamespaceData: OK\n")
		}
	}

	// DAS API
	if stats, err := client.DAS.SamplingStats(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "✗ DAS.SamplingStats failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ DAS.SamplingStats: HeadOfSampledChain=%d\n", stats.HeadOfSampledChain)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	if err := client.DAS.WaitCatchUp(waitCtx); err != nil && err != context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "✗ DAS.WaitCatchUp failed: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ DAS.WaitCatchUp: OK\n")
	}

	// Blob API
	if head, err := client.Header.LocalHead(ctx); err == nil {
		if blobs, err := client.Blob.GetAll(ctx, head.Height(), []share.Namespace{namespace}); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Blob.GetAll failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ Blob.GetAll: Count=%d\n", len(blobs))
		}
		if _, err := client.Blob.Included(ctx, head.Height(), namespace, []share.Namespace{namespace}); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Blob.Included failed: %v\n", err)
			hasErrors = true
		} else {
			fmt.Printf("✓ Blob.Included: OK\n")
		}
	}

	if hasErrors {
		fmt.Fprintf(os.Stderr, "✗ Cross-version compatibility test FAILED\n")
		os.Exit(1)
	}

	fmt.Println("✓ Cross-version compatibility test PASSED")
}

func hasProtocol(addr string) bool {
	return len(addr) > 7 && (addr[:7] == "http://" || addr[:8] == "https://")
}
