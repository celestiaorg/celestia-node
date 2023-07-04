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
	Arabica:        "7A5FABB19713D732D967B1DA84FA0DF5E87A7B62302D783F78743E216C1A3550",
	Mocha:          "1181AF8EAE5DDF3CBBFF3BF3CC44C5B795DF5094F5A0CC0AE52921ECCA0AF3C8",
	BlockspaceRace: "1A8491A72F73929680DAA6C93E3B593579261B2E76536BFA4F5B97D6FE76E088",
	Private:        "",
}
