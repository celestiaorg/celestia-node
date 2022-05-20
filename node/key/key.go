package key

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	KeyringAccName string
}

func DefaultConfig() Config {
	return Config{
		KeyringAccName: "",
	}
}
