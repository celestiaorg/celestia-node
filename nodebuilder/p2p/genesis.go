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
	Mainnet: "6BE39EFD10BA412A9DB5288488303F5DD32CF386707A5BEF33617F4C43301872",
	Arabica: "27122593765E07329BC348E8D16E92DCB4C75B34CCCB35C640FD7A4484D4C711",
	Mocha:   "8D8763ECEF33C0861CBEC98D5BCCD5FD71C09FCC3A980C52AEF8A4AFFCE8BA74",
	Private: "",
}
