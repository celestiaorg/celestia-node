package params

import "fmt"

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
	Arabica: "FAAA4B939173B4F8567E812FDC570FF186355326879017A611C58267A5D6EF17",
	Mamaki:  "41BBFD05779719E826C4D68C4CCBBC84B2B761EB52BC04CFDE0FF8603C9AA3CA",
	Private: "",
}
