package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

type response struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

var rpcClient *client.Client

var blobCmd = &cobra.Command{
	Use:   "blob [command]",
	Short: "Allows to interact with the Blob Service via JSON-RPC",
	Args:  cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client, err := getRPCClient(cmd.Context())
		if err != nil {
			return err
		}

		rpcClient = client
		return nil
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		rpcClient.Close()
	},
}

var getCmd = &cobra.Command{
	Use:   "Get [params]",
	Args:  cobra.ExactArgs(3),
	Short: "Returns the blob for the given namespace by commitment at a particular height.",
	Run: func(cmd *cobra.Command, args []string) {
		num, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr,
				fmt.Errorf("error parsing height: uint64 could not be parsed"),
			)
			os.Exit(1)
		}

		// 2. Namespace
		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr,
				fmt.Errorf("error parsing namespace: %v", err),
			)
			os.Exit(1)
		}

		// 3: Commitment
		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr,
				errors.New("error decoding commitment: base64 string could not be decoded"),
			)
			os.Exit(1)
		}

		blob, err := rpcClient.Blob.Get(cmd.Context(), num, namespace, commitment)
		if err == nil {
			if data, decodeErr := tryDecode(blob.Data); decodeErr == nil {
				blob.Data = data
			}
		}

		output := prepareOutput(blob, err)
		fmt.Fprintln(os.Stdout, string(output))
	},
}

var getAllCmd = &cobra.Command{
	Use:   "GetAll [params]",
	Args:  cobra.ExactArgs(2),
	Short: "Returns all blobs for the given namespace at a particular height.",
	Run: func(cmd *cobra.Command, args []string) {
		num, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr,
				fmt.Errorf("error parsing height: uint64 could not be parsed"),
			)
			os.Exit(1)
		}

		// 2. Namespace
		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr,
				fmt.Errorf("error parsing namespace: %v", err),
			)
			os.Exit(1)
		}

		blobs, err := rpcClient.Blob.GetAll(cmd.Context(), num, []share.Namespace{namespace})
		if err == nil {
			for _, b := range blobs {
				if data, err := tryDecode(b.Data); err == nil {
					b.Data = data
				}
			}
		}

		output := prepareOutput(blobs, err)
		fmt.Fprintln(os.Stdout, string(output))
	},
}

var submitCmd = &cobra.Command{
	Use:   "Submit [params]",
	Args:  cobra.ExactArgs(2),
	Short: "Submit the blob at the given namespace. Note: only one blob is allowed to submit through the RPC.",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Namespace
		namespace, err := parseV0Namespace(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr,
				fmt.Errorf("error parsing namespace: %v", err),
			)
			os.Exit(1)
		}

		// 2. Blob data
		var blobData []byte
		switch {
		case strings.HasPrefix(args[1], "0x"):
			decoded, err := hex.DecodeString(args[1][2:])
			if err != nil {
				fmt.Fprintln(os.Stderr, errors.New("error decoding blob: hex string could not be decoded"))
				os.Exit(1)
			}
			blobData = decoded
		case strings.HasPrefix(args[1], "\""):
			// user input an utf string that needs to be encoded to base64
			src := []byte(args[1])
			blobData = make([]byte, base64.StdEncoding.EncodedLen(len(src)))
			base64.StdEncoding.Encode(blobData, []byte(args[1]))
		default:
			// otherwise, we assume the user has already encoded their input to base64
			blobData, err = base64.StdEncoding.DecodeString(args[1])
			if err != nil {
				fmt.Fprintln(os.Stderr, errors.New("error decoding blob data: base64 string could not be decoded"))
				os.Exit(1)
			}
		}

		parsedBlob, err := blob.NewBlobV0(namespace, blobData)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("error creating blob: %v", err))
			os.Exit(1)
		}

		height, err := rpcClient.Blob.Submit(cmd.Context(), []*blob.Blob{parsedBlob})
		output := prepareOutput(height, err)

		fmt.Fprintln(os.Stdout, string(output))
	},
}

var getProofCmd = &cobra.Command{
	Use:   "GetProof [params]",
	Args:  cobra.ExactArgs(3),
	Short: "Retrieves the blob in the given namespaces at the given height by commitment and returns its Proof.",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Height
		num, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("error parsing height: uint64 could not be parsed"))
			os.Exit(1)
		}

		// 2. NamespaceID
		namespace, err := parseV0Namespace(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("error parsing namespace: %v", err))
			os.Exit(1)
		}

		// 3: Commitment
		commitment, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, errors.New("error decoding commitment: base64 string could not be decoded"))
		}

		proof, err := rpcClient.Blob.GetProof(cmd.Context(), num, namespace, commitment)
		output := prepareOutput(proof, err)
		fmt.Fprintln(os.Stdout, string(output))
	},
}

func prepareOutput(data interface{}, err error) []byte {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	bytes, err := json.MarshalIndent(response{
		Result: data,
		Error:  errStr,
	}, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return bytes
}

// Golang marshals `[]byte` as a base64-encoded string, so without additional encoding data will
// not match the expectations. We need this functionality in order to get the original data.
func tryDecode(data []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(data))
}

func getRPCClient(ctx context.Context) (*client.Client, error) {
	if authTokenFlag == "" {
		authTokenFlag = os.Getenv(authEnvKey)
	}

	return client.NewClient(ctx, requestURL, authTokenFlag)
}
