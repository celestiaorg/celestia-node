package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	base64Flag bool

	gasPrice float64

	// flagFileInput allows the user to provide file path to the json file
	// for submitting multiple blobs.
	flagFileInput = "input-file"
)

func init() {
	Cmd.AddCommand(getCmd, getAllCmd, submitCmd, getProofCmd)

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

	submitCmd.PersistentFlags().Float64Var(
		&gasPrice,
		"gas.price",
		float64(blob.DefaultGasPrice()),
		"specifies gas price (in utia) for blob submission.\n"+
			"Gas price will be set to default (0.002) if no value is passed",
	)

	submitCmd.PersistentFlags().String(flagFileInput, "", "Specify the file input")
}

var Cmd = &cobra.Command{
	Use:               "blob [command]",
	Short:             "Allows to interact with the Blob Service via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var getCmd = &cobra.Command{
	Use:   "get [height] [namespace] [commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Returns the blob for the given namespace by commitment at a particular height.",
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

		namespace, err := cmdnode.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace: %w", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment: %w", err)
		}

		blob, err := client.Blob.Get(cmd.Context(), height, namespace, commitment)

		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}
		return cmdnode.PrintOutput(blob, err, formatter)
	},
}

var getAllCmd = &cobra.Command{
	Use:   "get-all [height] [namespace]",
	Args:  cobra.ExactArgs(2),
	Short: "Returns all blobs for the given namespace at a particular height.",
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

		namespace, err := cmdnode.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace: %w", err)
		}

		blobs, err := client.Blob.GetAll(cmd.Context(), height, []share.Namespace{namespace})
		formatter := formatData
		if base64Flag || err != nil {
			formatter = nil
		}
		return cmdnode.PrintOutput(blobs, err, formatter)
	},
}

var submitCmd = &cobra.Command{
	Use: "submit [namespace] [blobData]",
	Args: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString(flagFileInput)
		if err != nil {
			return err
		}

		// If there is a file path input we'll check for the file extension
		if path != "" {
			if filepath.Ext(path) != ".json" {
				return fmt.Errorf("invalid file extension, require json got %s", filepath.Ext(path))
			}

			return nil
		}

		if len(args) < 2 {
			return errors.New("submit requires two arguments: namespace and blobData")
		}

		return nil
	},
	Short: "Submit the blob(s) at the given namespace(s).\n" +
		"User can use namespace and blobData as argument for single blob submission \n" +
		"or use --input-file flag with the path to a json file for multiple blobs submission, \n" +
		`where the json file contains: 

		{
			"Blobs": [
				{
					"namespace": "0x00010203040506070809",
					"blobData": "0x676d"
				},
				{
					"namespace": "0x42690c204d39600fddd3",
					"blobData": "0x676d"
				}
			]
		}` +
		"Note:\n" +
		"* fee and gas limit params will be calculated automatically.\n",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		path, err := cmd.Flags().GetString(flagFileInput)
		if err != nil {
			return err
		}

		jsonBlobs := make([]blobJSON, 0)
		// In case of there is a file input, get the namespace and blob from the arguments
		if path != "" {
			paresdBlobs, err := parseSubmitBlobs(path)
			if err != nil {
				return err
			}

			jsonBlobs = append(jsonBlobs, paresdBlobs...)
		} else {
			jsonBlobs = append(jsonBlobs, blobJSON{Namespace: args[0], BlobData: args[1]})
		}

		var blobs []*blob.Blob
		var commitments []blob.Commitment
		for _, jsonBlob := range jsonBlobs {
			blob, err := getBlobFromArguments(jsonBlob.Namespace, jsonBlob.BlobData)
			if err != nil {
				return err
			}
			blobs = append(blobs, blob)
			commitments = append(commitments, blob.Commitment)
		}

		height, err := client.Blob.Submit(
			cmd.Context(),
			blobs,
			blob.GasPrice(gasPrice),
		)

		response := struct {
			Height      uint64            `json:"height"`
			Commitments []blob.Commitment `json:"commitments"`
		}{
			Height:      height,
			Commitments: commitments,
		}
		return cmdnode.PrintOutput(response, err, nil)
	},
}

func getBlobFromArguments(namespaceArg, blobArg string) (*blob.Blob, error) {
	namespace, err := cmdnode.ParseV0Namespace(namespaceArg)
	if err != nil {
		return nil, fmt.Errorf("error parsing a namespace: %w", err)
	}

	parsedBlob, err := blob.NewBlobV0(namespace, []byte(blobArg))
	if err != nil {
		return nil, fmt.Errorf("error creating a blob: %w", err)
	}

	return parsedBlob, nil
}

var getProofCmd = &cobra.Command{
	Use:   "get-proof [height] [namespace] [commitment]",
	Args:  cobra.ExactArgs(3),
	Short: "Retrieves the blob in the given namespaces at the given height by commitment and returns its Proof.",
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

		namespace, err := cmdnode.ParseV0Namespace(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a namespace: %w", err)
		}

		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			return fmt.Errorf("error parsing a commitment: %w", err)
		}

		proof, err := client.Blob.GetProof(cmd.Context(), height, namespace, commitment)
		return cmdnode.PrintOutput(proof, err, nil)
	},
}

func formatData(data interface{}) interface{} {
	type tempBlob struct {
		Namespace    []byte `json:"namespace"`
		Data         string `json:"data"`
		ShareVersion uint32 `json:"share_version"`
		Commitment   []byte `json:"commitment"`
		Index        int    `json:"index"`
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
				Index:        b.Index(),
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
		Index:        b.Index(),
	}
}
