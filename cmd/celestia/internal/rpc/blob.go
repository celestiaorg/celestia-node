package rpc

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/cmd/celestia/internal"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	base64Flag bool

	fee      int64
	gasLimit uint64
)

func init() {
	BlobCmd.PersistentFlags().StringVar(
		&internal.RequestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)
	BlobCmd.AddCommand(getCmd, getAllCmd, submitCmd, getProofCmd)

	getCmd.PersistentFlags().BoolVar(
		&base64Flag,
		"base64",
		false,
		"printed blob's data a base64 string",
	)

	getAllCmd.PersistentFlags().BoolVar(
		&base64Flag,
		"base64",
		false,
		"printed blob's data as a base64 string",
	)

	submitCmd.PersistentFlags().Int64Var(
		&fee,
		"fee",
		-1,
		"specifies fee for blob submission",
	)

	submitCmd.PersistentFlags().Uint64Var(
		&gasLimit,
		"gas.limit",
		0,
		"specifies max gas for the blob submission",
	)
}

var BlobCmd = &cobra.Command{
	Use:               "blob [command]",
	Short:             "Allows to interact with the Blob Service via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: internal.InitClient,
	PersistentPostRun: internal.CloseClient,
}

var getCmd = &cobra.Command{
	Use:   "get [height, namespace, commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Returns the blob for the given namespace by commitment at a particular height.",
	RunE: func(cmd *cobra.Command, args []string) error {
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := internal.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment:%v", err)
		}

		blob, err := internal.RPCClient.Blob.Get(cmd.Context(), height, namespace, commitment)

		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}
		return internal.PrintOutput(blob, err, formatter)
	},
}

var getAllCmd = &cobra.Command{
	Use:   "get-all [height, namespace]",
	Args:  cobra.ExactArgs(2),
	Short: "Returns all blobs for the given namespace at a particular height.",
	RunE: func(cmd *cobra.Command, args []string) error {
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := internal.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		blobs, err := internal.RPCClient.Blob.GetAll(cmd.Context(), height, []share.Namespace{namespace})

		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}
		return internal.PrintOutput(blobs, err, formatter)
	},
}

var submitCmd = &cobra.Command{
	Use:   "submit [namespace, blobData]",
	Args:  cobra.ExactArgs(2),
	Short: "Submit the blob at the given namespace. Note: only one blob is allowed to submit through the RPC.",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, err := internal.ParseV0Namespace(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		parsedBlob, err := blob.NewBlobV0(namespace, []byte(args[1]))
		if err != nil {
			return fmt.Errorf("error creating a blob:%v", err)
		}

		height, err := internal.RPCClient.Blob.Submit(
			cmd.Context(),
			[]*blob.Blob{parsedBlob},
			&blob.SubmitOptions{Fee: fee, GasLimit: gasLimit},
		)

		response := struct {
			Height     uint64          `json:"height"`
			Commitment blob.Commitment `json:"commitment"`
		}{
			Height:     height,
			Commitment: parsedBlob.Commitment,
		}
		return internal.PrintOutput(response, err, nil)
	},
}

var getProofCmd = &cobra.Command{
	Use:   "get-proof [height, namespace, commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Retrieves the blob in the given namespaces at the given height by commitment and returns its Proof.",
	RunE: func(cmd *cobra.Command, args []string) error {
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := internal.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment:%v", err)
		}

		proof, err := internal.RPCClient.Blob.GetProof(cmd.Context(), height, namespace, commitment)
		return internal.PrintOutput(proof, err, nil)
	},
}

func formatData(data interface{}) interface{} {
	type tempBlob struct {
		Namespace    []byte `json:"namespace"`
		Data         string `json:"data"`
		ShareVersion uint32 `json:"share_version"`
		Commitment   []byte `json:"commitment"`
	}

	if reflect.TypeOf(data).Kind() == reflect.Slice {
		blobs := data.([]*blob.Blob)
		result := make([]tempBlob, len(blobs))
		for i, b := range blobs {
			result[i] = tempBlob{
				Namespace:    b.Namespace(),
				Data:         string(b.Data),
				ShareVersion: b.ShareVersion,
				Commitment:   b.Commitment,
			}
		}
		return result
	}

	b := data.(*blob.Blob)
	return tempBlob{
		Namespace:    b.Namespace(),
		Data:         string(b.Data),
		ShareVersion: b.ShareVersion,
		Commitment:   b.Commitment,
	}
}
