package p2p

import (
	"fmt"
)

// GenesisFor reports a hash of a genesis block for a given network.
// Genesis is strictly defined and can't be modified.
func GenesisFor(net Network) (string, error) {
	var err error
	net, err = net.Validate()
	if err != nil {
		return "", err
	}

	genHash, ok := genesisList[net]
	if !ok {
		return "", fmt.Errorf("params: genesis hash not found for network %s", net)
	}

	return genHash, nil
}

// NOTE: Every time we add a new long-running network, its genesis hash has to be added here.
var genesisList = map[Network]string{
	Arabica: "6D2D85AC2817EC2CCA8C75893EFA398B7EEB988528AC403CD0102AE4230D415A",
	Mocha:   "8038B21032C941372ED601699857043C12E5CC7D5945DCEEA4567D11B5712526",
	Private: "",
}
