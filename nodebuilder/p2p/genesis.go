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
	Arabica:        "EE310062CBB13CE98CBC7EAD3F6A827F0E4A86043FDEB2DA42048821877FE45C",
	Mocha:          "8038B21032C941372ED601699857043C12E5CC7D5945DCEEA4567D11B5712526",
	BlockspaceRace: "1A8491A72F73929680DAA6C93E3B593579261B2E76536BFA4F5B97D6FE76E088",
	Private:        "",
}
