package rpc

type Config struct {
	Address string
	Port    string
}

func DefaultConfig() Config {
	return Config{
		Address: "localhost",
		// do NOT expose the same port as celestia-core by default so that both can run on the same machine
		Port: "26658",
	}
}
