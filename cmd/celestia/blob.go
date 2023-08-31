package main

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

var base64Flag bool

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

		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}

		printOutput(blob, err, formatter)
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

		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}

		printOutput(blobs, err, formatter)
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

		height, err := client.Blob.Submit(cmd.Context(), []*blob.Blob{parsedBlob})

		response := struct {
			Height     uint64          `json:"uint64"`
			Commitment blob.Commitment `json:"commitment"`
		}{
			Height:     height,
			Commitment: parsedBlob.Commitment,
		}

		printOutput(response, err, nil)
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

		printOutput(proof, err, nil)
		return nil
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
