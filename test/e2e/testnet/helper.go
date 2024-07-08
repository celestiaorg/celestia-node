package testnet

import (
	"encoding/json"
	"fmt"
	"regexp"
)

func authTokenFromAuth(auth string) (string, error) {
	// Use regex to match the JWT token
	re := regexp.MustCompile(`[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_=]+\.?[A-Za-z0-9\-_=]*`)
	match := re.FindString(auth)

	return fmt.Sprintf(match), nil
}

func iDFromP2PInfo(p2pInfo string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(p2pInfo), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling status: %w", err)
	}
	resultData := result["result"].(map[string]interface{})
	id := resultData["id"].(string)
	return id, nil
}
