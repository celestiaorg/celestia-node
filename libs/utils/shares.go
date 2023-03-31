package utils

import (
	"github.com/celestiaorg/celestia-app/pkg/shares"
)

func AppSharesFromBytes(bytes [][]byte) (result []shares.Share, err error) {
	for _, share := range bytes {
		appShare, err := shares.NewShare(share)
		if err != nil {
			return nil, err
		}
		result = append(result, *appShare)
	}
	return result, nil
}
