package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	options "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	blob_api "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/share"
)

type response struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

var blobCmd = &cobra.Command{
	Use:   "blob [method] [params...]",
	Short: "Allow to interact with the Blob Service via JSON-RPC",
	Args:  cobra.MinimumNArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		blobApi := client.Modules["blob"]
		methods := reflect.VisibleFields(reflect.TypeOf(blobApi).Elem())
		var methodNames []string
		for _, m := range methods {
			methodNames = append(methodNames, m.Name+"\t"+parseSignatureForHelpstring(m))
		}
		return methodNames, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getRPCClient(cmd.Context())
		if err != nil {
			panic(err)
		}
		defer client.Close()

		var requestFn func([]string, *blob_api.API) (interface{}, error)

		// switch over method names
		switch args[0] {
		default:
			panic("invalid method requested")
		case "Submit":
			requestFn = handleSubmit
		case "Get":
			requestFn = handleGet
		case "GetAll":
			requestFn = handleGetAll
		case "GetProof":
			requestFn = handleGetProof
		}

		output := prepareOutput(requestFn(args[1:], &client.Blob))
		fmt.Println(string(output))
	},
}

func handleGet(params []string, client *blob_api.API) (interface{}, error) {
	// 1. Height
	num, err := strconv.ParseUint(params[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing height: uint64 could not be parsed")
	}

	// 2. Namespace
	namespace, err := parseV0Namespace(params[1])
	if err != nil {
		return nil, fmt.Errorf("error parsing namespace: %v", err)
	}

	// 3: Commitment
	commitment, err := base64.StdEncoding.DecodeString(params[2])
	if err != nil {
		return nil, errors.New("error decoding commitment: base64 string could not be decoded")
	}

	blob, err := client.Get(context.Background(), num, namespace, commitment)
	if err != nil {
		return nil, err
	}

	if data, err := tryDecode(blob.Data); err == nil {
		blob.Data = data
	}
	return blob, nil
}

func handleGetAll(params []string, client *blob_api.API) (interface{}, error) {
	// 1. Height
	num, err := strconv.ParseUint(params[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing height: uint64 could not be parsed")
	}

	// 2. Namespace
	namespace, err := parseV0Namespace(params[1])
	if err != nil {
		return nil, fmt.Errorf("error parsing namespace: %v", err)
	}

	blobs, err := client.GetAll(context.Background(), num, []share.Namespace{namespace})
	if err != nil {
		return nil, fmt.Errorf("error getting blobs: %s", err)
	}

	for _, b := range blobs {
		if data, err := tryDecode(b.Data); err == nil {
			b.Data = data
		}
	}
	return blobs, err
}

func handleSubmit(params []string, client *blob_api.API) (interface{}, error) {
	// 1. Namespace
	var err error
	namespace, err := parseV0Namespace(params[0])
	if err != nil {
		panic(fmt.Sprintf("Error parsing namespace: %v", err))
	}

	// 2. Blob data
	var blobData []byte
	switch {
	case strings.HasPrefix(params[1], "0x"):
		decoded, err := hex.DecodeString(params[1][2:])
		if err != nil {
			return nil, errors.New("error decoding blob: hex string could not be decoded")
		}
		blobData = decoded
	case strings.HasPrefix(params[1], "\""):
		// user input an utf string that needs to be encoded to base64
		src := []byte(params[1])
		blobData = make([]byte, base64.StdEncoding.EncodedLen(len(src)))
		base64.StdEncoding.Encode(blobData, []byte(params[1]))
	default:
		// otherwise, we assume the user has already encoded their input to base64
		blobData, err = base64.StdEncoding.DecodeString(params[1])
		if err != nil {
			return nil, errors.New("error decoding blob data: base64 string could not be decoded")
		}
	}

	parsedBlob, err := blob.NewBlobV0(namespace, blobData)
	if err != nil {
		return nil, fmt.Errorf("error creating blob: %v", err)
	}
	return client.Submit(context.Background(), []*blob.Blob{parsedBlob})
}

func handleGetProof(params []string, client *blob_api.API) (interface{}, error) {
	// 1. Height
	num, err := strconv.ParseUint(params[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing height: uint64 could not be parsed")
	}

	// 2. NamespaceID
	namespace, err := parseV0Namespace(params[1])
	if err != nil {
		return nil, fmt.Errorf("error parsing namespace: %v", err)
	}

	// 3: Commitment
	commitment, err := base64.StdEncoding.DecodeString(params[2])
	if err != nil {
		return nil, errors.New("error decoding commitment: base64 string could not be decoded")
	}
	return client.GetProof(context.Background(), num, namespace, commitment)
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
		panic(err)
	}
	return bytes
}

// Golang marshals `[]byte` as a base64-encoded string, so without additional encoding data will
// match expectations. We need this functionality in order to get the original data.
func tryDecode(data []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(data))
}

func getRPCClient(ctx context.Context) (*client.Client, error) {
	expanded, err := homedir.Expand(filepath.Clean(options.StorePath(ctx)))
	if err != nil {
		return nil, err
	}

	store, err := nodebuilder.OpenStoreReadOnly(expanded)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	cfg, err := store.Config()
	if err != nil {
		return nil, err
	}
	addr := cfg.RPC.Address
	port := cfg.RPC.Port
	listenAddr := "http://" + addr + ":" + port

	if authTokenFlag == "" {
		authTokenFlag = os.Getenv(authEnvKey)
	}
	return client.NewClient(ctx, listenAddr, authTokenFlag)
}
