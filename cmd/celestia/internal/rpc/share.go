package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/cmd/celestia/internal"
	"github.com/celestiaorg/celestia-node/share"
)

func init() {
	ShareCmd.PersistentFlags().StringVar(
		&internal.RequestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)

	ShareCmd.AddCommand(
		sharesAvailableCmd,
		probabilityOfAvailabilityCmd,
		getSharesByNamespaceCmd,
		getShare,
		getEDS,
	)
}

var ShareCmd = &cobra.Command{
	Use:               "share [command]",
	Short:             "Allows interaction with the Share Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: internal.InitClient,
	PersistentPostRun: internal.CloseClient,
}

var sharesAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "Subjectively validates if Shares committed to the given Root are available on the Network.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		err = internal.RPCClient.Share.SharesAvailable(cmd.Context(), &root)
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
		return internal.PrintOutput(err, nil, formatter)
	},
}

var probabilityOfAvailabilityCmd = &cobra.Command{
	Use:   "availability",
	Short: "Calculates the probability of the data square being available based on the number of samples collected.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		prob := internal.RPCClient.Share.ProbabilityOfAvailability(cmd.Context())
		return internal.PrintOutput(prob, nil, nil)
	},
}

var getSharesByNamespaceCmd = &cobra.Command{
	Use:   "get-by-namespace [dah, namespace]",
	Short: "Gets all shares from an EDS within the given namespace.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		ns, err := internal.ParseV0Namespace(args[1])
		if err != nil {
			return err
		}

		shares, err := internal.RPCClient.Share.GetSharesByNamespace(cmd.Context(), &root, ns)
		return internal.PrintOutput(shares, err, nil)
	},
}

var getShare = &cobra.Command{
	Use:   "get-share [dah, row, col]",
	Short: "Gets a Share by coordinates in EDS.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		s, err := internal.RPCClient.Share.GetShare(cmd.Context(), &root, int(row), int(col))

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
		return internal.PrintOutput(s, err, formatter)
	},
}

var getEDS = &cobra.Command{
	Use:   "get-eds [dah]",
	Short: "Gets the full EDS identified by the given root",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := parseJSON(args[0])
		if err != nil {
			return err
		}

		root := da.MinDataAvailabilityHeader()
		err = json.Unmarshal(raw, &root)
		if err != nil {
			return err
		}

		shares, err := internal.RPCClient.Share.GetEDS(cmd.Context(), &root)
		return internal.PrintOutput(shares, err, nil)
	},
}

func parseJSON(param string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := json.Unmarshal([]byte(param), &raw)
	return raw, err
}
