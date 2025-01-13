package cmd

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	Cmd.AddCommand(
		localHeadCmd,
		networkHeadCmd,
		getByHashCmd,
		getByHeightCmd,
		syncStateCmd,
	)
}

var Cmd = &cobra.Command{
	Use:               "header [command]",
	Short:             "Allows interaction with the Header Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var localHeadCmd = &cobra.Command{
	Use:   "local-head",
	Short: "Returns the ExtendedHeader from the chain head.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		header, err := client.Header.LocalHead(cmd.Context())
		return cmdnode.PrintOutput(header, err, nil)
	},
}

var networkHeadCmd = &cobra.Command{
	Use:   "network-head",
	Short: "Provides the Syncer's view of the current network head.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		header, err := client.Header.NetworkHead(cmd.Context())
		return cmdnode.PrintOutput(header, err, nil)
	},
}

var getByHashCmd = &cobra.Command{
	Use:   "get-by-hash",
	Short: "Returns the header of the given hash from the node's header store.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		hash, err := hex.DecodeString(args[0])
		if err != nil {
			return fmt.Errorf("error decoding a hash: expected a hex encoded string: %w", err)
		}
		header, err := client.Header.GetByHash(cmd.Context(), hash)
		return cmdnode.PrintOutput(header, err, nil)
	},
}

var getByHeightCmd = &cobra.Command{
	Use:   "get-by-height",
	Short: "Returns the ExtendedHeader at the given height if it is currently available.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height: %w", err)
		}

		header, err := client.Header.GetByHeight(cmd.Context(), height)
		return cmdnode.PrintOutput(header, err, nil)
	},
}

var syncStateCmd = &cobra.Command{
	Use:   "sync-state",
	Short: "Returns the current state of the header Syncer.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		header, err := client.Header.SyncState(cmd.Context())
		return cmdnode.PrintOutput(header, err, nil)
	},
}
