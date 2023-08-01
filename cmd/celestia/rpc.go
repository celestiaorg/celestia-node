package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

const (
	authEnvKey = "CELESTIA_NODE_AUTH_TOKEN"
)

var requestURL string
var authTokenFlag string
var printRequest bool

type jsonRPCRequest struct {
	ID      int64         `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type outputWithRequest struct {
	Request  jsonRPCRequest
	Response json.RawMessage
}

func init() {
	rpcCmd.PersistentFlags().StringVar(
		&requestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)
	rpcCmd.PersistentFlags().StringVar(
		&authTokenFlag,
		"auth",
		"",
		"Authorization token (if not provided, the "+authEnvKey+" environment variable will be used)",
	)
	rpcCmd.PersistentFlags().BoolVar(
		&printRequest,
		"print-request",
		false,
		"Print JSON-RPC request along with the response",
	)
	rootCmd.AddCommand(rpcCmd)
}

var rpcCmd = &cobra.Command{
	Use:   "rpc [namespace] [method] [params...]",
	Short: "Send JSON-RPC request",
	Args:  cobra.MinimumNArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		modules := client.Modules
		if len(args) == 0 {
			// get keys from modules (map[string]interface{})
			var keys []string
			for k := range modules {
				keys = append(keys, k)
			}
			return keys, cobra.ShellCompDirectiveNoFileComp
		} else if len(args) == 1 {
			// get methods from module
			module := modules[args[0]]
			methods := reflect.VisibleFields(reflect.TypeOf(module).Elem())
			var methodNames []string
			for _, m := range methods {
				methodNames = append(methodNames, m.Name+"\t"+parseSignatureForHelpstring(m))
			}
			return methodNames, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		namespace := args[0]
		method := args[1]
		params := parseParams(method, args[2:])

		sendJSONRPCRequest(namespace, method, params)
	},
}

func parseParams(method string, params []string) []interface{} {
	parsedParams := make([]interface{}, len(params))

	switch method {
	case "GetSharesByNamespace":
		// 1. Share Root
		root, err := parseJSON(params[0])
		if err != nil {
			panic(fmt.Errorf("couldn't parse share root as json: %v", err))
		}
		parsedParams[0] = root
		// 2. Namespace
		namespace, err := parseV0Namespace(params[1])
		if err != nil {
			panic(fmt.Sprintf("Error parsing namespace: %v", err))
		}
		parsedParams[1] = namespace
	case "Submit":
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
				panic("Error decoding blob: hex string could not be decoded.")
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
				panic("Error decoding blob data: base64 string could not be decoded.")
			}
		}
		parsedBlob, err := blob.NewBlobV0(namespace, blobData)
		if err != nil {
			panic(fmt.Sprintf("Error creating blob: %v", err))
		}
		parsedParams[0] = []*blob.Blob{parsedBlob}
		// param count doesn't match input length, so cut off nil values
		return parsedParams[:1]
	case "SubmitPayForBlob":
		// 1. Fee (state.Int is a string)
		parsedParams[0] = params[0]
		// 2. GasLimit (uint64)
		num, err := strconv.ParseUint(params[1], 10, 64)
		if err != nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[1] = num
		// 3. Namespace
		namespace, err := parseV0Namespace(params[2])
		if err != nil {
			panic(fmt.Sprintf("Error parsing namespace: %v", err))
		}
		// 4. Blob data
		var blobData []byte
		switch {
		case strings.HasPrefix(params[3], "0x"):
			decoded, err := hex.DecodeString(params[3][2:])
			if err != nil {
				panic("Error decoding blob: hex string could not be decoded.")
			}
			blobData = decoded
		case strings.HasPrefix(params[3], "\""):
			// user input an utf string that needs to be encoded to base64
			src := []byte(params[1])
			blobData = make([]byte, base64.StdEncoding.EncodedLen(len(src)))
			base64.StdEncoding.Encode(blobData, []byte(params[3]))
		default:
			// otherwise, we assume the user has already encoded their input to base64
			blobData, err = base64.StdEncoding.DecodeString(params[3])
			if err != nil {
				panic("Error decoding blob: base64 string could not be decoded.")
			}
		}
		parsedBlob, err := blob.NewBlobV0(namespace, blobData)
		if err != nil {
			panic(fmt.Sprintf("Error creating blob: %v", err))
		}
		parsedParams[2] = []*blob.Blob{parsedBlob}
		return parsedParams[:3]
	case "Get":
		// 1. Height
		num, err := strconv.ParseUint(params[0], 10, 64)
		if err != nil {
			panic("Error parsing height: uint64 could not be parsed.")
		}
		parsedParams[0] = num
		// 2. NamespaceID
		namespace, err := parseV0Namespace(params[1])
		if err != nil {
			panic(fmt.Sprintf("Error parsing namespace: %v", err))
		}
		parsedParams[1] = namespace
		// 3: Commitment
		commitment, err := base64.StdEncoding.DecodeString(params[2])
		if err != nil {
			panic("Error decoding commitment: base64 string could not be decoded.")
		}
		parsedParams[2] = commitment
		return parsedParams
	case "GetAll": // NOTE: Over the cli, you can only pass one namespace
		// 1. Height
		num, err := strconv.ParseUint(params[0], 10, 64)
		if err != nil {
			panic("Error parsing height: uint64 could not be parsed.")
		}
		parsedParams[0] = num
		// 2. Namespace
		namespace, err := parseV0Namespace(params[1])
		if err != nil {
			panic(fmt.Sprintf("Error parsing namespace: %v", err))
		}
		parsedParams[1] = []share.Namespace{namespace}
		return parsedParams
	case "QueryDelegation", "QueryUnbonding", "BalanceForAddress":
		var err error
		parsedParams[0], err = parseAddressFromString(params[0])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		return parsedParams
	case "QueryRedelegations":
		var err error
		parsedParams[0], err = parseAddressFromString(params[0])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		parsedParams[1], err = parseAddressFromString(params[1])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		return parsedParams
	case "Transfer", "Delegate", "Undelegate":
		// 1. Address
		var err error
		parsedParams[0], err = parseAddressFromString(params[0])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		// 2. Amount + Fee
		parsedParams[1] = params[1]
		parsedParams[2] = params[2]
		// 3. GasLimit (uint64)
		num, err := strconv.ParseUint(params[3], 10, 64)
		if err != nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[3] = num
		return parsedParams
	case "CancelUnbondingDelegation":
		// 1. Validator Address
		var err error
		parsedParams[0], err = parseAddressFromString(params[0])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		// 2. Amount + Height + Fee
		parsedParams[1] = params[1]
		parsedParams[2] = params[2]
		parsedParams[3] = params[3]
		// 4. GasLimit (uint64)
		num, err := strconv.ParseUint(params[4], 10, 64)
		if err != nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[4] = num
	case "BeginRedelegate":
		// 1. Source Validator Address
		var err error
		parsedParams[0], err = parseAddressFromString(params[0])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		// 2. Destination Validator Address
		parsedParams[1], err = parseAddressFromString(params[1])
		if err != nil {
			panic(fmt.Errorf("error parsing address: %w", err))
		}
		// 2. Amount + Fee
		parsedParams[2] = params[2]
		parsedParams[3] = params[3]
		// 4. GasLimit (uint64)
		num, err := strconv.ParseUint(params[4], 10, 64)
		if err != nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[4] = num
	default:
	}

	for i, param := range params {
		if param[0] == '{' || param[0] == '[' {
			rawJSON, err := parseJSON(param)
			if err != nil {
				parsedParams[i] = param
			} else {
				parsedParams[i] = rawJSON
			}
		} else {
			// try to parse arguments as numbers before adding them as strings
			num, err := strconv.ParseInt(param, 10, 64)
			if err == nil {
				parsedParams[i] = num
				continue
			}
			parsedParams[i] = param
		}
	}

	return parsedParams
}

func sendJSONRPCRequest(namespace, method string, params []interface{}) {
	url := requestURL
	request := jsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  fmt.Sprintf("%s.%s", namespace, method),
		Params:  params,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Error marshaling JSON-RPC request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Fatalf("Error creating JSON-RPC request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	authToken := authTokenFlag
	if authToken == "" {
		authToken = os.Getenv(authEnvKey)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending JSON-RPC request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err) //nolint:gocritic
	}

	rawResponseJSON, err := parseJSON(string(responseBody))
	if err != nil {
		log.Fatalf("Error parsing JSON-RPC response: %v", err)
	}
	if printRequest {
		output, err := json.MarshalIndent(outputWithRequest{
			Request:  request,
			Response: rawResponseJSON,
		}, "", "  ")
		if err != nil {
			panic(fmt.Sprintf("Error marshaling JSON-RPC response: %v", err))
		}
		fmt.Println(string(output))
		return
	}

	output, err := json.MarshalIndent(rawResponseJSON, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Error marshaling JSON-RPC response: %v", err))
	}
	fmt.Println(string(output))
}

func parseAddressFromString(addrStr string) (state.Address, error) {
	var address state.Address
	err := address.UnmarshalJSON([]byte(addrStr))
	if err != nil {
		return address, err
	}
	return address, nil
}

func parseSignatureForHelpstring(methodSig reflect.StructField) string {
	simplifiedSignature := "("
	in, out := methodSig.Type.NumIn(), methodSig.Type.NumOut()
	for i := 1; i < in; i++ {
		simplifiedSignature += methodSig.Type.In(i).String()
		if i != in-1 {
			simplifiedSignature += ", "
		}
	}
	simplifiedSignature += ") -> ("
	for i := 0; i < out-1; i++ {
		simplifiedSignature += methodSig.Type.Out(i).String()
		if i != out-2 {
			simplifiedSignature += ", "
		}
	}
	simplifiedSignature += ")"
	return simplifiedSignature
}

// parseV0Namespace parses a namespace from a base64 or hex string. The param
// is expected to be the user-specified portion of a v0 namespace ID (i.e. the
// last 10 bytes).
func parseV0Namespace(param string) (share.Namespace, error) {
	userBytes, err := decodeToBytes(param)
	if err != nil {
		return nil, err
	}

	// if the namespace ID is <= 10 bytes, left pad it with 0s
	return share.NewBlobNamespaceV0(userBytes)
}

// decodeToBytes decodes a Base64 or hex input string into a byte slice.
func decodeToBytes(param string) ([]byte, error) {
	if strings.HasPrefix(param, "0x") {
		decoded, err := hex.DecodeString(param[2:])
		if err != nil {
			return nil, fmt.Errorf("error decoding namespace ID: %w", err)
		}
		return decoded, nil
	}
	// otherwise, it's just a base64 string
	decoded, err := base64.StdEncoding.DecodeString(param)
	if err != nil {
		return nil, fmt.Errorf("error decoding namespace ID: %w", err)
	}
	return decoded, nil
}
func parseJSON(param string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := json.Unmarshal([]byte(param), &raw)
	return raw, err
}
