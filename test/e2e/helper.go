package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"

	"github.com/tendermint/tendermint/rpc/client/http"
)

func queryTxCount(ctx context.Context, rpcAddr string) (int, error) {
	blockchain, err := testnode.ReadBlockchain(ctx, rpcAddr)
	if err != nil {
		return 0, err
	}

	totalTxs := 0
	for _, block := range blockchain {
		totalTxs += len(block.Data.Txs)
	}
	return totalTxs, nil
}

func waitForTxs(ctx context.Context, rpcAddr string, expectedTxs int, logger *log.Logger) error {
	ticker := time.NewTicker(queryTxCountInterval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		totalTxs, err := queryTxCount(ctx, rpcAddr)
		if err != nil {
			return err
		}

		if totalTxs >= expectedTxs {
			logger.Printf("Found %d transactions", totalTxs)
			return nil
		}
		logger.Printf("Waiting for at least %d transactions, got %d so far", expectedTxs, totalTxs)
	}
	return nil
}

func getHeight(ctx context.Context, client *http.HTTP, period time.Duration) (int64, error) {
	timer := time.NewTimer(period)
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-timer.C:
			return 0, fmt.Errorf("failed to get height after %.2f seconds", period.Seconds())
		case <-ticker.C:
			status, err := client.Status(ctx)
			if err == nil {
				return status.SyncInfo.LatestBlockHeight, nil
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return 0, err
			}
		}
	}
}
