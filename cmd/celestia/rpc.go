package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/state"
)

const (
	authEnvKey = "CELESTIA_NODE_AUTH_TOKEN"
)

var requestURL string
var authTokenFlag string

type jsonRPCRequest struct {
	ID      int64         `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
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
				methodNames = append(methodNames, m.Name)
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
		parsedParams[0] = params[0]
		// 2. NamespaceID
		if strings.HasPrefix(params[1], "0x") {
			decoded, err := hex.DecodeString(params[1][2:])
			if err != nil {
				panic("Error decoding namespace ID: hex string could not be decoded.")
			}
			parsedParams[1] = decoded
		} else {
			// otherwise, it's just a base64 string
			parsedParams[1] = params[1]
		}
		return parsedParams
	case "SubmitPayForBlob":
		// 1. NamespaceID
		if strings.HasPrefix(params[0], "0x") {
			decoded, err := hex.DecodeString(params[0][2:])
			if err != nil {
				panic("Error decoding namespace ID: hex string could not be decoded.")
			}
			parsedParams[0] = decoded
		} else {
			// otherwise, it's just a base64 string
			parsedParams[0] = params[0]
		}
		// 2. Blob
		switch {
		case strings.HasPrefix(params[1], "0x"):
			decoded, err := hex.DecodeString(params[1][2:])
			if err != nil {
				panic("Error decoding blob: hex string could not be decoded.")
			}
			parsedParams[0] = decoded
		case strings.HasPrefix(params[1], "\""):
			// user input an utf string that needs to be encoded to base64
			parsedParams[1] = base64.StdEncoding.EncodeToString([]byte(params[1]))
		default:
			// otherwise, we assume the user has already encoded their input to base64
			parsedParams[1] = params[1]
		}
		// 3. Fee (state.Int is a string)
		parsedParams[2] = params[2]
		// 4. GasLimit (uint64)
		num, err := strconv.ParseUint(params[3], 10, 64)
		if err != nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[3] = num
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
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(param), &raw); err == nil {
				parsedParams[i] = raw
			} else {
				parsedParams[i] = param
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

	fmt.Println(string(responseBody))
}

func parseAddressFromString(addrStr string) (state.Address, error) {
	var addr state.AccAddress
	addr, err := types.AccAddressFromBech32(addrStr)
	if err != nil {
		// first check if it is a validator address and can be converted
		valAddr, err := types.ValAddressFromBech32(addrStr)
		if err != nil {
			return nil, errors.New("address must be a valid account or validator address ")
		}
		return valAddr, nil
	}

	return addr, nil
}
