package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	libshare "github.com/celestiaorg/go-square/v4/share"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
)

func init() {
	Cmd.AddCommand(
		sharesAvailableCmd,
		getSharesByNamespaceCmd,
		getShare,
		getEDS,
		getRange,
		shrexPeersCmd,
	)
}

// parseIndex parses s as a non-negative int (a share index or dimension).
// It rejects negative values and values that overflow int on the current platform.
func parseIndex(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("must be non-negative, got %d", n)
	}
	return n, nil
}

var Cmd = &cobra.Command{
	Use:               "share [command]",
	Short:             "Allows interaction with the Share Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var sharesAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "Subjectively validates if Shares committed to the given EDS are available on the Network.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}

		err = client.Share.SharesAvailable(cmd.Context(), height)
		formatter := func(data any) any {
			err, ok := data.(error)
			available := !ok

			return struct {
				Available bool   `json:"available"`
				Hash      []byte `json:"dah_hash"`
				Reason    error  `json:"reason,omitempty"`
			}{
				Available: available,
				Hash:      []byte(args[0]),
				Reason:    err,
			}
		}
		return cmdnode.PrintOutput(err, nil, formatter)
	},
}

var getSharesByNamespaceCmd = &cobra.Command{
	Use:   "get-by-namespace (height | hash) namespace",
	Short: "Gets all shares from an EDS within the given namespace.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}

		ns, err := cmdnode.ParseV0Namespace(args[1])
		if err != nil {
			return err
		}

		shares, err := client.Share.GetNamespaceData(cmd.Context(), height, ns)
		return cmdnode.PrintOutput(shares, err, nil)
	},
}

var getShare = &cobra.Command{
	Use:   "get-share (height | hash) row col",
	Short: "Gets a Share by coordinates in EDS.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}

		row, err := parseIndex(args[1])
		if err != nil {
			return fmt.Errorf("row: %w", err)
		}

		col, err := parseIndex(args[2])
		if err != nil {
			return fmt.Errorf("col: %w", err)
		}

		s, err := client.Share.GetShare(cmd.Context(), height, row, col)

		formatter := func(data any) any {
			sh, ok := data.(libshare.Share)
			if !ok {
				return data
			}

			ns := hex.EncodeToString(sh.Namespace().Bytes())

			return struct {
				Namespace string `json:"namespace"`
				Data      []byte `json:"data"`
			}{
				Namespace: ns,
				Data:      sh.RawData(),
			}
		}
		return cmdnode.PrintOutput(s, err, formatter)
	},
}

var getEDS = &cobra.Command{
	Use:   "get-eds (height | hash)",
	Short: "Gets the full EDS identified by the given block height",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}

		shares, err := client.Share.GetEDS(cmd.Context(), height)
		return cmdnode.PrintOutput(shares, err, nil)
	},
}

var shrexPeersCmd = &cobra.Command{
	Use:   "shrex-peers [peer-id-filter]",
	Short: "Shows shrex peer manager pools with per-peer scores, stats, in-flight/downloading state and cooldowns",
	Long: "Shows a diagnostic snapshot of the shrex peer managers (one per discovery tag, e.g. " +
		"\"full\" and \"archival\"). For each managed peer it reports the selection score, quality, " +
		"success/latency estimates, cumulative served counts, whether the peer is currently downloading, " +
		"whether it hit an in-flight or rate limit, and any active cooldown. An optional argument filters " +
		"peers whose ID contains the given substring.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		snaps, err := client.Share.ShrexPeers(cmd.Context())
		if err != nil {
			return cmdnode.PrintOutput(nil, err, nil)
		}

		if len(args) == 1 {
			snaps = filterShrexPeers(snaps, args[0])
		}
		return cmdnode.PrintOutput(snaps, nil, nil)
	},
}

// filterShrexPeers keeps only peers whose ID contains substr in each manager's nodes
// pool, and drops the per-datahash pool summaries which are not peer-specific.
func filterShrexPeers(snaps []peers.ManagerSnapshot, substr string) []peers.ManagerSnapshot {
	out := make([]peers.ManagerSnapshot, 0, len(snaps))
	for _, s := range snaps {
		filtered := s.Nodes.Peers[:0:0]
		for _, p := range s.Nodes.Peers {
			if strings.Contains(p.ID, substr) {
				filtered = append(filtered, p)
			}
		}
		s.Nodes.Peers = filtered
		s.DataHashPools = nil
		out = append(out, s)
	}
	return out
}

var getRange = &cobra.Command{
	Use:   "get-range [height] [start] [end(exclusive)]",
	Short: "Gets a range of shares from the given height within the given ODS indexes",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}
		start, err := parseIndex(args[1])
		if err != nil {
			return fmt.Errorf("start: %w", err)
		}
		end, err := parseIndex(args[2])
		if err != nil {
			return fmt.Errorf("end: %w", err)
		}

		if start >= end {
			return errors.New("start index must be less than end index")
		}

		rng, err := client.Share.GetRange(cmd.Context(), height, start, end)
		return cmdnode.PrintOutput(rng, err, nil)
	},
}
