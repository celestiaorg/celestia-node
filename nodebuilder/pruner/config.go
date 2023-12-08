package pruner

var MetricsEnabled bool

type Config struct {
	EnableService bool
}

func DefaultConfig() Config {
	return Config{
		EnableService: false,
	}
}
