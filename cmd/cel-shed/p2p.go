package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func init() {
	p2pCmd.AddCommand(p2pNewKeyCmd, p2pPeerIDCmd, p2pConnectBootstrappersCmd)
}

var p2pCmd = &cobra.Command{
	Use:   "p2p [subcommand]",
	Short: "Collection of p2p related utilities",
}

var p2pNewKeyCmd = &cobra.Command{
	Use:   "new-key",
	Short: "Generate and print new Ed25519 private key for p2p networking",
	RunE: func(_ *cobra.Command, _ []string) error {
		privkey, _, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return err
		}

		raw, err := privkey.Raw()
		if err != nil {
			return err
		}

		fmt.Println(hex.EncodeToString(raw))
		return nil
	},
	Args: cobra.NoArgs,
}

var p2pPeerIDCmd = &cobra.Command{
	Use:   "peer-id",
	Short: "Get peer-id out of public or private Ed25519 key",
	RunE: func(_ *cobra.Command, args []string) error {
		decKey, err := hex.DecodeString(args[0])
		if err != nil {
			return err
		}

		privKey, err := crypto.UnmarshalEd25519PrivateKey(decKey)
		if err != nil {
			// try pubkey then
			pubKey, err := crypto.UnmarshalEd25519PublicKey(decKey)
			if err != nil {
				return err
			}

			id, err := peer.IDFromPublicKey(pubKey)
			if err != nil {
				return err
			}

			fmt.Println(id.String())
			return nil
		}

		id, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			return err
		}

		fmt.Println(id.String())
		return nil
	},
	Args: cobra.ExactArgs(1),
}

// RTTStats represents the statistics from multiple RTT measurements
type RTTStats struct {
	PeerID  string        `json:"peer_id"`
	Address string        `json:"address,omitempty"`
	Count   int           `json:"count"`
	Success int           `json:"success"`
	Failed  int           `json:"failed"`
	Min     time.Duration `json:"-"` // Use custom marshaling
	Max     time.Duration `json:"-"` // Use custom marshaling
	Average time.Duration `json:"-"` // Use custom marshaling
	Median  time.Duration `json:"-"` // Use custom marshaling
	StdDev  time.Duration `json:"-"` // Use custom marshaling
	Results []RTTResult   `json:"results,omitempty"`

	// String versions for JSON output
	MinStr     string `json:"min,omitempty"`
	MaxStr     string `json:"max,omitempty"`
	AverageStr string `json:"average,omitempty"`
	MedianStr  string `json:"median,omitempty"`
	StdDevStr  string `json:"std_dev,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for RTTStats
func (s *RTTStats) MarshalJSON() ([]byte, error) {
	type Alias RTTStats

	// Create a copy with string versions of durations
	if s.Success > 0 {
		s.MinStr = s.Min.String()
		s.MaxStr = s.Max.String()
		s.AverageStr = s.Average.String()
		s.MedianStr = s.Median.String()
		s.StdDevStr = s.StdDev.String()
	}

	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
}

// RTTResult represents a single RTT measurement result
type RTTResult struct {
	Attempt int           `json:"attempt"`
	RTT     time.Duration `json:"rtt,omitempty"`
	Error   string        `json:"error,omitempty"`
}

var (
	errorOnAnyFailure bool
	errorOnAllFailure bool
	connectionTimeout time.Duration
	pingCount         int
	pingInterval      time.Duration
	jsonOutput        bool
	showDetailedPings bool
)

var p2pConnectBootstrappersCmd = &cobra.Command{
	Use:   "connect-bootstrappers [network]",
	Short: "Connect to bootstrappers of a certain network and measure RTT latency",
	RunE: func(cmd *cobra.Command, args []string) error {
		if errorOnAnyFailure && errorOnAllFailure {
			return fmt.Errorf("only one of --err-any and --err-all can be specified")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), connectionTimeout)
		defer cancel()

		network := p2p.GetNetwork(args[0])
		bootstrappers, err := p2p.BootstrappersFor(network)
		if err != nil {
			return fmt.Errorf("failed to get bootstrappers: %w", err)
		}

		store := nodebuilder.NewMemStore()
		cfg := p2p.DefaultConfig(node.Light)
		modp2p := p2p.ConstructModule(node.Light, &cfg)

		var mod p2p.Module
		app := fx.New(
			fx.NopLogger,
			modp2p,
			fx.Provide(fraud.Unmarshaler),
			fx.Provide(cmd.Context),
			fx.Provide(store.Keystore),
			fx.Provide(store.Datastore),
			fx.Supply(bootstrappers),
			fx.Supply(network),
			fx.Supply(node.Light),
			fx.Invoke(func(modprov p2p.Module) {
				mod = modprov
			}),
		)

		if err := app.Start(ctx); err != nil {
			return fmt.Errorf("failed to start app: %w", err)
		}
		defer func() {
			// Create a new context with a longer timeout for shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := app.Stop(shutdownCtx); err != nil {
				fmt.Printf("failed to stop application: %v\n", err)
			}
		}()

		p2pInfo, err := mod.Info(ctx)
		if err != nil {
			return fmt.Errorf("failed to get p2p info: %w", err)
		}

		if !jsonOutput {
			fmt.Printf("PeerID: %s\n", p2pInfo.ID)
			for _, addr := range p2pInfo.Addrs {
				fmt.Printf("Listening on: %s\n", addr.String())
			}
			fmt.Println()
		}

		var successfulConnections atomic.Int32
		var failedConnections atomic.Int32
		var wg sync.WaitGroup
		rttStatsMu := sync.Mutex{}
		rttStats := make(map[string]*RTTStats)

		for _, bootstrapper := range bootstrappers {
			wg.Add(1)
			go func(bootstrapper peer.AddrInfo) {
				defer wg.Done()

				if !jsonOutput {
					fmt.Printf("Attempting to connect to bootstrapper: %s\n", bootstrapper)
				}

				if err := mod.Connect(ctx, bootstrapper); err != nil {
					if !jsonOutput {
						fmt.Printf("Error: Failed to connect to bootstrapper %s. Reason: %v\n", bootstrapper, err)
					}
					failedConnections.Add(1)
					return
				}

				if !jsonOutput {
					fmt.Printf("Success: Connected to bootstrapper: %s\n", bootstrapper)
				}

				successfulConnections.Add(1)

				// Get the address string and strip the /dnsaddr/ prefix if present
				addrStr := bootstrapper.Addrs[0].String()
				if len(addrStr) > 9 && addrStr[:9] == "/dnsaddr/" {
					addrStr = addrStr[9:]
				}

				// Perform RTT measurements
				stats := &RTTStats{
					PeerID:  bootstrapper.ID.String(),
					Address: addrStr,
					Count:   pingCount,
					Min:     time.Duration(math.MaxInt64),
					Max:     0,
					Results: []RTTResult{},
				}

				rttValues := []time.Duration{}

				for i := 0; i < pingCount; i++ {
					result := RTTResult{Attempt: i + 1}

					// Wait for the specified interval between pings
					if i > 0 {
						time.Sleep(pingInterval)
					}

					// Create a context with timeout for each ping
					pingCtx, pingCancel := context.WithTimeout(ctx, connectionTimeout)

					// Measure RTT using the Ping method
					rtt, pingErr := mod.Ping(pingCtx, bootstrapper.ID)
					pingCancel()

					if pingErr != nil {
						result.Error = pingErr.Error()
						stats.Failed++
					} else {
						result.RTT = rtt
						stats.Success++

						// Update min/max
						if rtt < stats.Min {
							stats.Min = rtt
						}
						if rtt > stats.Max {
							stats.Max = rtt
						}

						rttValues = append(rttValues, rtt)
					}

					stats.Results = append(stats.Results, result)
				}

				// Calculate statistics if we have successful pings
				if len(rttValues) > 0 {
					// Calculate average
					var sum time.Duration
					for _, rtt := range rttValues {
						sum += rtt
					}
					stats.Average = sum / time.Duration(len(rttValues))

					// Calculate median
					sort.Slice(rttValues, func(i, j int) bool {
						return rttValues[i] < rttValues[j]
					})

					if len(rttValues)%2 == 0 {
						// Even number of samples, average the middle two
						middle := len(rttValues) / 2
						stats.Median = (rttValues[middle-1] + rttValues[middle]) / 2
					} else {
						// Odd number of samples, take the middle one
						stats.Median = rttValues[len(rttValues)/2]
					}

					// Calculate standard deviation
					var sumSquares float64
					avg := float64(stats.Average)
					for _, rtt := range rttValues {
						diff := float64(rtt) - avg
						sumSquares += diff * diff
					}
					stdDev := math.Sqrt(sumSquares / float64(len(rttValues)))
					stats.StdDev = time.Duration(stdDev)

					// If min was never set (all pings failed), set it to 0
					if stats.Min == time.Duration(math.MaxInt64) {
						stats.Min = 0
					}
				} else {
					// If all pings failed, set min to 0
					stats.Min = 0
				}

				// If not showing detailed results, clear them to save space
				if !showDetailedPings {
					stats.Results = nil
				}

				rttStatsMu.Lock()
				rttStats[bootstrapper.ID.String()] = stats
				rttStatsMu.Unlock()

			}(bootstrapper)
		}

		wg.Wait()

		if !jsonOutput {
			fmt.Println("\nRTT Measurement Results:")
			fmt.Println("========================")

			for _, stats := range rttStats {
				fmt.Printf("\nBootstrapper: %s\n", stats.PeerID)
				fmt.Printf("Address: %s\n", stats.Address)
				fmt.Printf("  Ping Count: %d (Success: %d, Failed: %d)\n", stats.Count, stats.Success, stats.Failed)

				if stats.Success > 0 {
					fmt.Printf("  Min RTT: %v\n", stats.Min)
					fmt.Printf("  Max RTT: %v\n", stats.Max)
					fmt.Printf("  Avg RTT: %v\n", stats.Average)
					fmt.Printf("  Median RTT: %v\n", stats.Median)
					fmt.Printf("  StdDev RTT: %v\n", stats.StdDev)

					if showDetailedPings && len(stats.Results) > 0 {
						fmt.Println("  Detailed Results:")
						for _, result := range stats.Results {
							if result.Error != "" {
								fmt.Printf("    Attempt %d: Failed - %s\n", result.Attempt, result.Error)
							} else {
								fmt.Printf("    Attempt %d: %v\n", result.Attempt, result.RTT)
							}
						}
					}
				} else {
					fmt.Println("  All ping attempts failed")
				}
			}
		} else {
			// Convert the map to a slice for consistent JSON output
			statsList := make([]*RTTStats, 0, len(rttStats))
			for _, stats := range rttStats {
				statsList = append(statsList, stats)
			}

			jsonData, err := json.MarshalIndent(statsList, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		}

		if failedConnections.Load() == int32(len(bootstrappers)) && errorOnAllFailure {
			fmt.Println()
			fmt.Println("failed to connect to all bootstrappers")
			os.Exit(1)
			return nil
		} else if failedConnections.Load() > 0 && errorOnAnyFailure {
			fmt.Println()
			fmt.Println("failed to connect to some bootstrappers")
			os.Exit(1)
			return nil
		}

		return nil
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	p2pConnectBootstrappersCmd.Flags().BoolVar(
		&errorOnAnyFailure, "err-any", false,
		"Return error if at least one bootstrapper is not reachable",
	)
	p2pConnectBootstrappersCmd.Flags().BoolVar(
		&errorOnAllFailure, "err-all", false,
		"Return error if no bootstrapper is reachable",
	)
	p2pConnectBootstrappersCmd.Flags().DurationVar(
		&connectionTimeout, "timeout", 10*time.Second,
		"Timeout for the connection attempt",
	)
	p2pConnectBootstrappersCmd.Flags().IntVar(
		&pingCount, "ping-count", 5,
		"Number of ping attempts per bootstrapper for RTT measurement",
	)
	p2pConnectBootstrappersCmd.Flags().DurationVar(
		&pingInterval, "ping-interval", 500*time.Millisecond,
		"Interval between ping attempts",
	)
	p2pConnectBootstrappersCmd.Flags().BoolVar(
		&jsonOutput, "json", false,
		"Output results in JSON format",
	)
	p2pConnectBootstrappersCmd.Flags().BoolVar(
		&showDetailedPings, "detailed", false,
		"Show detailed results for each ping attempt",
	)
}
