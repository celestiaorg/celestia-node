package state

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

// ValidateBasic performs basic validation of the config.
func (cfg *Config) ValidateBasic() error {
	return nil
}
