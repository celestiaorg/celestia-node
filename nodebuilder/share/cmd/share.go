package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
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
	Short: "Subjectively validates if Shares committed to the given Root are available on the Network.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		eh, err := getExtendedHeaderFromCmdArg(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}

		err = client.Share.SharesAvailable(cmd.Context(), eh)
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

		eh, err := getExtendedHeaderFromCmdArg(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}

		ns, err := cmdnode.ParseV0Namespace(args[1])
		if err != nil {
			return err
		}

		shares, err := client.Share.GetSharesByNamespace(cmd.Context(), eh, ns)
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

		eh, err := getExtendedHeaderFromCmdArg(cmd.Context(), client, args[0])
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

		s, err := client.Share.GetShare(cmd.Context(), eh, int(row), int(col))

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

		eh, err := getExtendedHeaderFromCmdArg(cmd.Context(), client, args[0])
		if err != nil {
			return err
		}

		shares, err := client.Share.GetEDS(cmd.Context(), eh)
		return cmdnode.PrintOutput(shares, err, nil)
	},
}

func getExtendedHeaderFromCmdArg(ctx context.Context, client *rpc.Client, arg string) (*header.ExtendedHeader, error) {
	height, err := strconv.ParseUint(arg, 10, 64)
	if err == nil {
		return client.Header.GetByHeight(ctx, height)
	}

	hash, err := hex.DecodeString(arg)
	if err != nil {
		return nil, fmt.Errorf("can't parse the height/hash argument: %w", err)
	}

	return client.Header.GetByHash(ctx, hash)
}
