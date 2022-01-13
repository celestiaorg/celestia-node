package build

// Genesis reports a hash of a genesis block for a current network.
func Genesis() string {
	return genesisList[network] // network is guarantee to be valid
}

// GenesisFor reports a hash of a genesis block for a given network.
func GenesisFor(net Network) (string, error) {
	if err := net.Validate(); err != nil {
		return "", err
	}

	return genesisList[net], nil
}

// NOTE: Everytime we add a new long-running network, it's genesis hash has to be added it here.
var genesisList = map[Network]string{
	DevNet: "4632277C441CA6155C4374AC56048CF4CFE3CBB2476E07A548644435980D5E17",
}
