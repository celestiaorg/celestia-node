package rpc

type Config struct {
	ListenAddr string
}

func DefaultConfig() Config {
	return Config{
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		ListenAddr: "0.0.0.0:26658",
	}
}
