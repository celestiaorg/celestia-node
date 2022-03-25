package params

import (
	"os"
	"strings"
)

// Genesis reports a hash of a genesis block for the current network.
func Genesis() string {
	return genesisList[network] // network is guaranteed to be valid
}

// GenesisFor reports a hash of a genesis block for a given network.
func GenesisFor(net Network) (string, error) {
	if err := net.Validate(); err != nil {
		return "", err
	}

	return genesisList[net], nil
}

// NOTE: Every time we add a new long-running network, its genesis hash has to be added here.
var genesisList = map[Network]string{
	DevNet:  "4632277C441CA6155C4374AC56048CF4CFE3CBB2476E07A548644435980D5E17",
	Private: "",
}

func init() {
	if genesis, ok := os.LookupEnv("CELESTIA_GENESIS_HASH"); ok {
		network = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
}
