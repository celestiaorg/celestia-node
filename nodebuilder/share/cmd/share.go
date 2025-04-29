package cmd

import (
	"encoding/hex"
	"strconv"

	"github.com/spf13/cobra"

	libshare "github.com/celestiaorg/go-square/v2/share"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	Cmd.AddCommand(
		sharesAvailableCmd,
		getSharesByNamespaceCmd,
		getShare,
		getEDS,
	)
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

		row, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}

		col, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return err
		}

		s, err := client.Share.GetShare(cmd.Context(), height, int(row), int(col))

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
