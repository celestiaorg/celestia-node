package prometheus

import "github.com/celestiaorg/knuu/pkg/errors"

var (
	ErrPrometheusAlreadyRunning       = errors.New("PrometheusAlreadyRunning", "prometheus instance is already running")
	ErrEmptyScrapeJobName             = errors.New("EmptyScrapeJobName", "scrape job name is empty")
	ErrEmptyScrapeJobTarget           = errors.New("EmptyScrapeJobTarget", "scrape job target is empty")
	ErrFailedToFetchPrometheusData    = errors.New("FailedToFetchPrometheusData", "failed to fetch prometheus data")
	ErrFailedToParsePrometheusMetrics = errors.New("FailedToParsePrometheusMetrics", "failed to parse prometheus metrics")
	ErrMetricNotFound                 = errors.New("MetricNotFound", "metric not found")
)
