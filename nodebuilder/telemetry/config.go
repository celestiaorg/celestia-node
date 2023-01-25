// This package hosts everything relative to general
// node telemetry and observability, namely configuration.
package telemetry

type Config struct {
	// Enabled defines whether telemetry is enabled or not.
	Enabled bool `toml:"enabled"`

	// NodeUptimeScrapeInterval defines the interval at which node uptime
	// metrics are scraped (in minutes)
	NodeUptimeScrapeInterval int `toml:"metrics_scrape_interval"`
}

var (
	// DefaultMetricsScrapeInterval defines the default interval at which metrics are scraped.
	// This is primarily used with the ticker that records
	// node uptime
	DefaultNodeUptimeScrapeInterval = 1
)

// DefaultConfig returns the default configuration for telemetry.
func DefaultConfig() Config {
	return Config{
		NodeUptimeScrapeInterval: DefaultNodeUptimeScrapeInterval,
	}
}
