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
	Arabica:        "5904E55478BA4B3002EE885621E007A2A6A2399662841912219AECD5D5CBE393",
	Mocha:          "79A97034D569C4199A867439B1B7B77D4E1E1D9697212755E1CE6D920CDBB541",
	BlockspaceRace: "1A8491A72F73929680DAA6C93E3B593579261B2E76536BFA4F5B97D6FE76E088",
	Private:        "",
}
