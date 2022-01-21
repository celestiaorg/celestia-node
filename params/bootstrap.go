package params

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

// NOTE: Every time we add a new long-running network, its bootstrap peers have to be added here.
var bootstrapList = map[Network][]string{
	DevNet: {}, // TODO(@Wondertan): Set multiaddress once bootstrap peers are set up.
}
