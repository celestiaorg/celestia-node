package main

import (
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
)

func init() {
	shareCmd.AddCommand(
		sharesAvailableCmd,
		probabilityOfAvailabilityCmd,
		getSharesByNamespaceCmd,
		getShare,
		getEDS,
	)
}

var shareCmd = &cobra.Command{
	Use:   "share [command]",
	Short: "Allows interaction with the Share Module via JSON-RPC",
	Args:  cobra.NoArgs,
}

var sharesAvailableCmd = &cobra.Command{
	Use:   "shares-available",
	Short: "Subjectively validates if Shares committed to the given Root are available on the Network.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		err = client.Share.SharesAvailable(cmd.Context(), &root)
		formatter := func(data interface{}) interface{} {
			err, ok := data.(error)
			available := false
			if !ok {
				available = true
			}
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
		return printOutput(err, nil, formatter)
	},
}

var probabilityOfAvailabilityCmd = &cobra.Command{
	Use:   "probability-of-availability",
	Short: "Calculates the probability of the data square being available based on the number of samples collected.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		prob := client.Share.ProbabilityOfAvailability(cmd.Context())
		return printOutput(prob, nil, nil)
	},
}

var getSharesByNamespaceCmd = &cobra.Command{
	Use:   "get-shares-by-namespace [dah, namespace]",
	Short: "Gets all shares from an EDS within the given namespace.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		ns, err := parseV0Namespace(args[1])
		if err != nil {
			return err
		}

		shares, err := client.Share.GetSharesByNamespace(cmd.Context(), &root, ns)
		return printOutput(shares, err, nil)
	},
}

var getShare = &cobra.Command{
	Use:   "get-share [dah, row, col]",
	Short: "Gets a Share by coordinates in EDS.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		row, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}

		col, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return err
		}

		s, err := client.Share.GetShare(cmd.Context(), &root, int(row), int(col))

		formatter := func(data interface{}) interface{} {
			sh, ok := data.(share.Share)
			if !ok {
				return data
			}

			ns := hex.EncodeToString(share.GetNamespace(sh))

			return struct {
				Namespace string `json:"namespace"`
				Data      []byte `json:"data"`
			}{
				Namespace: ns,
				Data:      share.GetData(sh),
			}
		}
		return printOutput(s, err, formatter)
	},
}

var getEDS = &cobra.Command{
	Use:   "get-eds [dah]",
	Short: "Gets the full EDS identified by the given root",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		shares, err := client.Share.GetEDS(cmd.Context(), &root)
		return printOutput(shares, err, nil)
	},
}
