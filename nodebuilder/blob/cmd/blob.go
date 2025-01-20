package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	state "github.com/celestiaorg/celestia-node/nodebuilder/state/cmd"
)

// flagFileInput allows the user to provide file path to the json file
// for submitting multiple blobs.
var flagFileInput = "input-file"

func init() {
	Cmd.AddCommand(getCmd, getAllCmd, submitCmd, getProofCmd)

	state.ApplyFlags(submitCmd)

	submitCmd.PersistentFlags().String(flagFileInput, "", "Specifies the file input")
}

var Cmd = &cobra.Command{
	Use:               "blob [command]",
	Short:             "Allows to interact with the Blob Service via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var getCmd = &cobra.Command{
	Use:  "get [height] [namespace] [commitment]",
	Args: cobra.ExactArgs(3),
	Short: "Returns the blob for the given namespace by commitment at a particular height.\n" +
		"Note:\n* Both namespace and commitment input parameters are expected to be in their hex representation.",
	PreRunE: func(_ *cobra.Command, args []string) error {
		if !strings.HasPrefix(args[0], "0x") {
			args[0] = "0x" + args[0]
		}
		if !strings.HasPrefix(args[1], "0x") {
			args[1] = "0x" + args[1]
		}
		return nil
	},
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

		commitment, err := hex.DecodeString(args[2][2:])
		if err != nil {
			return fmt.Errorf("error parsing a commitment: %w", err)
		}

		blob, err := client.Blob.Get(cmd.Context(), height, namespace, commitment)
		return cmdnode.PrintOutput(blob, err, formatData(args[1]))
	},
}

var getAllCmd = &cobra.Command{
	Use:  "get-all [height] [namespace]",
	Args: cobra.ExactArgs(2),
	Short: "Returns all blobs for the given namespace at a particular height.\n" +
		"Note:\n* Namespace input parameter is expected to be in its hex representation.",
	PreRunE: func(_ *cobra.Command, args []string) error {
		if !strings.HasPrefix(args[0], "0x") {
			args[0] = "0x" + args[0]
		}
		if !strings.HasPrefix(args[1], "0x") {
			args[1] = "0x" + args[1]
		}
		return nil
	},
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

		blobs, err := client.Blob.GetAll(cmd.Context(), height, []libshare.Namespace{namespace})
		return cmdnode.PrintOutput(blobs, err, formatData(args[1]))
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
	PreRunE: func(_ *cobra.Command, args []string) error {
		if !strings.HasPrefix(args[0], "0x") {
			args[0] = "0x" + args[0]
		}
		if !strings.HasPrefix(args[1], "0x") {
			args[1] = "0x" + args[1]
		}
		return nil
	},
	Short: "Submit the blob(s) at the given namespace(s) and " +
		"returns the header height in which the blob(s) was/were include + the respective commitment(s).\n" +
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
		"* Namespace input parameter is expected to be its their hex representation.\n" +
		"* Commitment(s) output parameter(s) will be in the hex representation.",
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

		var resultBlobs []*blob.Blob
		var commitments []string
		for _, jsonBlob := range jsonBlobs {
			blob, err := getBlobFromArguments(jsonBlob.Namespace, jsonBlob.BlobData)
			if err != nil {
				return err
			}
			resultBlobs = append(resultBlobs, blob)
			hexedCommitment := hex.EncodeToString(blob.Commitment)
			commitments = append(commitments, "0x"+hexedCommitment)
		}

		height, err := client.Blob.Submit(
			cmd.Context(),
			resultBlobs,
			state.GetTxConfig(),
		)

		response := struct {
			Height      uint64   `json:"height"`
			Commitments []string `json:"commitments"`
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
	Use:  "get-proof [height] [namespace] [commitment]",
	Args: cobra.ExactArgs(3),
	Short: "Retrieves the blob in the given namespaces at the given height by commitment and returns its Proof.\n" +
		"Note:\n* Both namespace and commitment input parameters are expected to be in their hex representation.",
	PreRunE: func(_ *cobra.Command, args []string) error {
		if !strings.HasPrefix(args[0], "0x") {
			args[0] = "0x" + args[0]
		}
		if !strings.HasPrefix(args[1], "0x") {
			args[1] = "0x" + args[1]
		}
		return nil
	},
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

		commitment, err := hex.DecodeString(args[2][2:])
		if err != nil {
			return fmt.Errorf("error parsing a commitment: %w", err)
		}

		proof, err := client.Blob.GetProof(cmd.Context(), height, namespace, commitment)
		return cmdnode.PrintOutput(proof, err, nil)
	},
}

func formatData(ns string) func(interface{}) interface{} {
	return func(data interface{}) interface{} {
		type tempBlob struct {
			Namespace    string `json:"namespace"`
			Data         string `json:"data"`
			ShareVersion uint32 `json:"share_version"`
			Commitment   string `json:"commitment"`
			Index        int    `json:"index"`
		}

		if reflect.TypeOf(data).Kind() == reflect.Slice {
			blobs := data.([]*blob.Blob)
			result := make([]tempBlob, len(blobs))
			for i, b := range blobs {
				result[i] = tempBlob{
					Namespace:    ns,
					Data:         string(b.Data()),
					ShareVersion: uint32(b.ShareVersion()),
					Commitment:   "0x" + hex.EncodeToString(b.Commitment),
					Index:        b.Index(),
				}
			}
			return result
		}

		b := data.(*blob.Blob)
		return tempBlob{
			Namespace:    ns,
			Data:         string(b.Data()),
			ShareVersion: uint32(b.ShareVersion()),
			Commitment:   "0x" + hex.EncodeToString(b.Commitment),
			Index:        b.Index(),
		}
	}
}
