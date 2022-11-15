package p2p

import (
	"fmt"
)

// GenesisFor reports a hash of a genesis block for a given network.
// Genesis is strictly defined and can't be modified.
func GenesisFor(net Network) (string, error) {
	if err := net.Validate(); err != nil {
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
	Arabica: "04EE55B212745B88F29943D7B9528C415473A211F12EEF6E9333EF32E4DEAF3C",
	Mamaki:  "41BBFD05779719E826C4D68C4CCBBC84B2B761EB52BC04CFDE0FF8603C9AA3CA",
	Private: "",
}
