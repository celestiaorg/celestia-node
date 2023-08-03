package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

type response struct {
	Result interface{} `json:"result"`
}

func init() {
	blobCmd.AddCommand(getCmd, getAllCmd, submitCmd, getProofCmd)
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

		output, err := prepareOutput(blob, err)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, string(output))
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

		var output []byte
		blobs, err := client.Blob.GetAll(cmd.Context(), height, []share.Namespace{namespace})

		output, err = prepareOutput(blobs, err)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, string(output))
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

		output, err := prepareOutput(response, err)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, string(output))
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

		output, err := prepareOutput(proof, err)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, string(output))
		return nil
	},
}

func prepareOutput(data interface{}, err error) ([]byte, error) {
	if err != nil {
		data = err
	}

	bytes, err := json.MarshalIndent(response{
		Result: data,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
