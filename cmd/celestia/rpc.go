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
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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
		"Authorization token (if not provided, the AUTH_TOKEN environment variable will be used)",
	)
	rootCmd.AddCommand(rpcCmd)
}

var rpcCmd = &cobra.Command{
	Use:   "rpc [namespace] [method] [params...]",
	Short: "Send JSON-RPC request",
	Args:  cobra.MinimumNArgs(2),
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
		if err == nil {
			panic("Error parsing gas limit: uint64 could not be parsed.")
		}
		parsedParams[3] = num
		return parsedParams
	default:
	}

	for i, param := range params {
		// try to parse arguments as numbers before adding them as strings
		num, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			parsedParams[i] = param
		}
		parsedParams[i] = num
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
		authToken = os.Getenv("AUTH_TOKEN")
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
