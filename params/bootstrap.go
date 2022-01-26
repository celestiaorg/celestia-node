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
	DevNet: {
		"/dns4/andromeda.celestia-devops.dev/tcp/2121/p2p/12D3KooWSv6aX4eweBMUtDBXSbTu2uvX1Nf7eFWsDgrJvrgduzU9",
		"/dns4/libra.celestia-devops.dev/tcp/2121/p2p/12D3KooWNGuNLG7Kiom4UUAyiW6pdJnPXQV7xGereokMJxB5f6SU",
		"/dns4/norma.celestia-devops.dev/tcp/2121/p2p/12D3KooWBKReVZa4bE1Edfn4TfxK2UCq3qqTtQSHiHeHQy18YgiY",
	},
}
