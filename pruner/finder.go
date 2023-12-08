package pruner

import "github.com/celestiaorg/celestia-node/header"

type checkpoint struct {
	lastPrunedHeader *header.ExtendedHeader
}

func (s *Service) findHeaders() ([]*header.ExtendedHeader, error) {
	// 1. 	take last pruned header timestamp
	// 2. 	estimate availability window last header based on time and block time
	// 		estimation (header height based on last header)
	// 3. 	load header, check timestamp
	// 4. 	begin algorithm to find header
}
