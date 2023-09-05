package main

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	headerCmd.AddCommand(
		localHeadCmd,
		networkHeadCmd,
		getByHashCmd,
		getByHeightCmd,
		syncStateCmd,
	)
}

var headerCmd = &cobra.Command{
	Use:   "header [command]",
	Short: "Allows interaction with the Header Module via JSON-RPC",
	Args:  cobra.NoArgs,
}

var localHeadCmd = &cobra.Command{
	Use:   "local-head",
	Short: "Returns the ExtendedHeader from the chain head",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		header, err := client.Header.LocalHead(cmd.Context())
		return printOutput(header, err, nil)
	},
}

var networkHeadCmd = &cobra.Command{
	Use:   "network-head",
	Short: "Returns the recent ExtendedHeader from the network",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		header, err := client.Header.NetworkHead(cmd.Context())
		return printOutput(header, err, nil)
	},
}

var getByHashCmd = &cobra.Command{
	Use:   "get-by-hash",
	Short: "Returns the ExtendedHeader by its hash",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		hash, err := hex.DecodeString(args[0])
		if err != nil {
			return err
		}
		header, err := client.Header.GetByHash(cmd.Context(), hash)
		return printOutput(header, err, nil)
	},
}

var getByHeightCmd = &cobra.Command{
	Use:   "get-by-height",
	Short: "Returns the ExtendedHeader from the particular height",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		header, err := client.Header.GetByHeight(cmd.Context(), height)
		return printOutput(header, err, nil)
	},
}

var syncStateCmd = &cobra.Command{
	Use:   "sync-state",
	Short: "Returns the current state of the Syncer",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		header, err := client.Header.SyncState(cmd.Context())
		return printOutput(header, err, nil)
	},
}
