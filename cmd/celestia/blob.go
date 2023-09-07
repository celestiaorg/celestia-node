package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	base64Flag bool

	fee      int64
	gasLimit uint64
)

func init() {
	blobCmd.AddCommand(getCmd, getAllCmd, submitCmd, getProofCmd)

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
		"gas-limit",
		0,
		"specifies max gas for the blob submission",
	)
}

var blobCmd = &cobra.Command{
	Use:   "blob [command]",
	Short: "Allows to interact with the Blob Service via JSON-RPC",
	Args:  cobra.NoArgs,
}

var getCmd = &cobra.Command{
	Use:   "get [height, namespace, commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Returns the blob for the given namespace by commitment at a particular height.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment:%v", err)
		}

		blob, err := client.Blob.Get(cmd.Context(), height, namespace, commitment)

		printOutput(blob, err)
		return nil
	},
}

var getAllCmd = &cobra.Command{
	Use:   "get-all [height, namespace]",
	Args:  cobra.ExactArgs(2),
	Short: "Returns all blobs for the given namespace at a particular height.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		blobs, err := client.Blob.GetAll(cmd.Context(), height, []share.Namespace{namespace})

		printOutput(blobs, err)
		return nil
	},
}

var submitCmd = &cobra.Command{
	Use:   "submit [namespace, blobData]",
	Args:  cobra.ExactArgs(2),
	Short: "Submit the blob at the given namespace. Note: only one blob is allowed to submit through the RPC.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		namespace, err := parseV0Namespace(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		parsedBlob, err := blob.NewBlobV0(namespace, []byte(args[1]))
		if err != nil {
			return fmt.Errorf("error creating a blob:%v", err)
		}

		height, err := client.Blob.Submit(
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

		printOutput(response, err)
		return nil
	},
}

var getProofCmd = &cobra.Command{
	Use:   "get-proof [height, namespace, commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Retrieves the blob in the given namespaces at the given height by commitment and returns its Proof.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a height:%v", err)
		}

		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment:%v", err)
		}

		proof, err := client.Blob.GetProof(cmd.Context(), height, namespace, commitment)

		printOutput(proof, err)
		return nil
	},
}

func printOutput(data interface{}, err error) {
	if err != nil {
		data = err
	}

	if !base64Flag && err == nil {
		data = formatData(data)
	}

	resp := struct {
		Result interface{} `json:"result"`
	}{
		Result: data,
	}

	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, string(bytes))
}

func formatData(data interface{}) interface{} {
	type tempBlob struct {
		Namespace    []byte `json:"namespace"`
		Data         string `json:"data"`
		ShareVersion uint32 `json:"share_version"`
		Commitment   []byte `json:"commitment"`
	}

	if reflect.TypeOf(data).Kind() == reflect.Slice {
		blobs, ok := data.([]*blob.Blob)
		if !ok {
			return data
		}

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

	b, ok := data.(*blob.Blob)
	if !ok {
		return data
	}
	return tempBlob{
		Namespace:    b.Namespace(),
		Data:         string(b.Data),
		ShareVersion: b.ShareVersion,
		Commitment:   b.Commitment,
	}
}
