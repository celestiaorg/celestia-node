package build

// Bootstrappers reports multiaddresses of bootstrap peers for the node's current network.
func Bootstrappers() []string {
	return bootstrapList[network] // network is guaranteed to be valid
}

// BootstrappersFor reports multiaddresses of bootstrap peers for a given network.
func BootstrappersFor(net Network) ([]string, error) {
	if err := net.Validate(); err != nil {
		return nil, err
	}

	return bootstrapList[net], nil
}

// NOTE: Everytime we add a new long-running network, it's genesis hash has to be added it here.
var bootstrapList = map[Network][]string{
	DevNet: {}, // TODO(@Wondertan): Set multiaddress once bootstrap peers are set up.
}
