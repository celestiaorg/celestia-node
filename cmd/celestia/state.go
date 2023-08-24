package main

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
)

var submitPFB = &cobra.Command{
	Use:   "submitPFB [namespace, data, fee, gasLim]",
	Short: "Allows to submit pfbs",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		namespace, err := parseV0Namespace(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}

		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gasLim:%v", err)
		}
		parsedBlob, err := blob.NewBlobV0(namespace, []byte(args[1]))
		if err != nil {
			return fmt.Errorf("error creating a blob:%v", err)
		}

		txResp, err := client.State.SubmitPayForBlob(cmd.Context(), types.NewInt(fee), gasLimit, []*blob.Blob{parsedBlob})
		printOutput(txResp, err)
		return nil
	},
}
