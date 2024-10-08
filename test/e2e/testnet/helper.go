package testnet

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

func authTokenFromAuth(auth string) string {
	// Use regex to match the JWT token
	re := regexp.MustCompile(`[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_=]+\.?[A-Za-z0-9\-_=]*`)
	match := re.FindString(auth)

	return match
}

func iDFromP2PInfo(p2pInfo string) (string, error) {
	var result map[string]interface{}

	if err := json.Unmarshal([]byte(p2pInfo), &result); err != nil {
		return "", ErrFailedToUnmarshalP2PInfo.Wrap(err)
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetResultFromP2PInfo
	}

	id, ok := resultData["id"].(string)
	if !ok {
		return "", ErrFailedToGetIDFromP2PInfo
	}
	return id, nil
}

func getTrustedPeers(ctx context.Context, trustedNode *Node) (string, error) {
	p2pInfoNode, err := trustedNode.Instance.Execution().
		ExecuteCommand(ctx, "celestia", "p2p", "info", "--node.store", remoteRootDir)
	if err != nil {
		return "", ErrFailedToGetP2PInfo.Wrap(err)
	}

	bridgeIP, err := trustedNode.Instance.Network().GetIP(ctx)
	if err != nil {
		return "", ErrFailedToGetIP.Wrap(err)
	}
	bridgeID, err := iDFromP2PInfo(p2pInfoNode)
	if err != nil {
		return "", ErrFailedToGetBridgeID.Wrap(err)
	}
	return fmt.Sprintf("/ip4/%s/tcp/2121/p2p/%s", bridgeIP, bridgeID), nil
}

func hashFromBlock(block string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(block), &result)
	if err != nil {
		return "", ErrFailedToUnmarshalBlock.Wrap(err)
	}
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetResultFromBlock
	}
	blockId, ok := resultData["block_id"].(map[string]interface{})
	if !ok {
		return "", ErrFailedToGetBlockIdFromBlock
	}
	blockHash, ok := blockId["hash"].(string)
	if !ok {
		return "", ErrFailedToGetHashFromBlockId
	}
	return blockHash, nil
}
