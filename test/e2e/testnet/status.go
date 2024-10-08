package testnet

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/celestiaorg/knuu/pkg/instance"
)

// Status is the status of the app
type Status struct {
	status []byte
}

func GetStatus(ctx context.Context, executor *instance.Instance, appIP string) (*Status, error) {
	if executor == nil {
		return nil, ErrExecutorNotSet
	}
	if appIP == "" {
		return nil, ErrEmptyAppIP
	}

	status, err := executor.Execution().ExecuteCommand(ctx, "wget", "-q", "-O", "-", fmt.Sprintf("%s:26657/status", appIP))
	if err != nil {
		return nil, ErrFailedToGetStatus.Wrap(err)
	}
	return &Status{status: []byte(status)}, nil
}

func (s *Status) String() string {
	return string(s.status)
}

func (s *Status) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.status)
}

func (s *Status) ChainID() (string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(s.status, &result); err != nil {
		return "", ErrFailedToUnmarshalStatus.Wrap(err)
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetResultFromStatus
	}

	nodeInfo, ok := resultData["node_info"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetNodeInfoFromStatus
	}

	chainId, ok := nodeInfo["network"].(string)
	if !ok {
		return "", ErrFailedToGetNetworkFromNodeInfo
	}
	return chainId, nil
}

func (s *Status) NodeID() (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal(s.status, &result)
	if err != nil {
		return "", ErrFailedToUnmarshalStatus.Wrap(err)
	}

	if errorField, ok := result["error"]; ok {
		errorData, ok := errorField.(map[string]interface{})
		if !ok {
			return "", ErrFailedToGetErrorDataFromStatus.
				Wrap(fmt.Errorf("error field exists but is not a map[string]interface{}"))
		}
		jsonError := &JSONRPCError{}
		if errorCode, ok := errorData["code"].(float64); ok {
			jsonError.Code = int(errorCode)
		}
		if errorMessage, ok := errorData["message"].(string); ok {
			jsonError.Message = errorMessage
		}
		if errorData, ok := errorData["data"].(string); ok {
			jsonError.Data = errorData
		}
		return "", jsonError
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetResultFromStatus
	}
	nodeInfo, ok := resultData["node_info"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetNodeInfoFromStatus
	}
	id, ok := nodeInfo["id"].(string)
	if !ok {
		return "", ErrFailedToGetIdFromNodeInfo
	}
	return id, nil
}

func (s *Status) LatestBlockHeight() (int64, error) {
	var result map[string]interface{}
	err := json.Unmarshal(s.status, &result)
	if err != nil {
		return 0, ErrFailedToUnmarshalStatus.Wrap(err)
	}

	if errorField, ok := result["error"]; ok {
		errorData, ok := errorField.(map[string]interface{})
		if !ok {
			return 0, ErrFailedToGetErrorDataFromStatus.
				Wrap(fmt.Errorf("error field exists but is not a map[string]interface{}"))
		}
		jsonError := &JSONRPCError{}
		if errorCode, ok := errorData["code"].(float64); ok {
			jsonError.Code = int(errorCode)
		}
		if errorMessage, ok := errorData["message"].(string); ok {
			jsonError.Message = errorMessage
		}
		if errorData, ok := errorData["data"].(string); ok {
			jsonError.Data = errorData
		}
		return 0, jsonError
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return 0, ErrFailedToGetResultFromStatus
	}
	syncInfo, ok := resultData["sync_info"].(map[string]interface{})
	if !ok {
		return 0, ErrFailedToGetSyncInfoFromStatus
	}
	latestBlockHeight, ok := syncInfo["latest_block_height"].(string)
	if !ok {
		return 0, ErrFailedToGetLatestBlockHeightFromSyncInfo
	}
	latestBlockHeightInt, err := strconv.ParseInt(latestBlockHeight, 10, 64)
	if err != nil {
		return 0, ErrFailedToConvertLatestBlockHeightToInt.Wrap(err)
	}
	return latestBlockHeightInt, nil
}
