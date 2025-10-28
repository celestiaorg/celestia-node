package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v6/pkg/da"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
)

func init() {
	squareCmd.AddCommand(constructFromCoreCmd)
}

var squareCmd = &cobra.Command{
	Use:   "square [subcommand]",
	Short: "Collection of square-related utilities",
}

var constructFromCoreCmd = &cobra.Command{
	Use:   "construct-from-core [core endpoint in ip:port format] [height]",
	Short: "Constructs a complete data square and extended header from a signed core block",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := grpc.NewClient(args[0], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(err)
		}

		fetcher, err := core.NewBlockFetcher(conn)
		if err != nil {
			panic(err)
		}

		height, err := strconv.Atoi(args[1])
		if err != nil {
			panic(err)
		}

		sb, err := fetcher.GetSignedBlock(cmd.Context(), int64(height))
		if err != nil {
			panic(err)
		}

		eds, err := da.ConstructEDS(sb.Data.Txs.ToSliceOfBytes(), sb.Header.Version.App, -1)
		if err != nil {
			return fmt.Errorf("extending block data: %w", err)
		}

		fmt.Printf("\nCONSTRUCTED EDS FROM BLOCK DATA AT HEIGHT %d...", height)
		fmt.Printf("\nEDS width:  %d", eds.Width())

		eh, err := header.MakeExtendedHeader(sb.Header, sb.Commit, sb.ValidatorSet, eds)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\nCONSTRUCTED EXTENDED HEADER FROM BLOCK DATA AT HEIGHT %d...", height)
		fmt.Printf("\nData hash in raw header:  %s", eh.DataHash.String())
		fmt.Printf("\nComputed data hash from EDS:  %s", eh.DAH.String())
		fmt.Println()

		return nil
	},
	Args: cobra.ExactArgs(2),
}
