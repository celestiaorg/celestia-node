package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"
)

const (
	instanceName          = "prometheus"
	image                 = "prom/prometheus:v2.49.0"
	apiPort               = 9090
	defaultScrapeInterval = 15 * time.Second
	prometheusConfig      = "/etc/prometheus/prometheus.yml"
	prometheusArgs        = "--config.file=" + prometheusConfig
	fileOwner             = "0:0"
)

type ScrapeJob struct {
	Name     string
	Target   string
	Interval time.Duration
}

type Prometheus struct {
	knuu                *knuu.Knuu
	instance            *instance.Instance
	prometheusProxyHost string
	scrapeJobs          []ScrapeJob
}

type MetricFilter struct {
	MetricName string
	Labels     map[string]string
}

func New(kn *knuu.Knuu) (*Prometheus, error) {
	ins, err := kn.NewInstance(instanceName)
	if err != nil {
		return nil, err
	}
	return &Prometheus{knuu: kn, instance: ins}, nil
}

func (p *Prometheus) AddScrapeJob(ctx context.Context, scrapeJob ScrapeJob) error {
	if p.instance.IsState(instance.StateStarted) {
		return ErrPrometheusAlreadyRunning
	}

	if scrapeJob.Name == "" {
		return ErrEmptyScrapeJobName
	}
	if scrapeJob.Target == "" {
		return ErrEmptyScrapeJobTarget
	}
	p.scrapeJobs = append(p.scrapeJobs, scrapeJob)
	return nil
}

func (p *Prometheus) Destroy(ctx context.Context) error {
	return p.instance.Execution().Destroy(ctx)
}

func (p *Prometheus) Start(ctx context.Context) error {
	if err := p.instance.Build().SetImage(ctx, image); err != nil {
		return err
	}

	if err := p.instance.Network().AddPortTCP(apiPort); err != nil {
		return err
	}

	if err := p.instance.Build().Commit(ctx); err != nil {
		return err
	}

	if err := p.instance.Storage().AddFileBytes([]byte(p.generateConfig()), prometheusConfig, fileOwner); err != nil {
		return err
	}

	if err := p.instance.Build().SetArgs(prometheusArgs); err != nil {
		return err
	}

	if err := p.instance.Execution().Start(ctx); err != nil {
		return err
	}

	proxyHost, err := p.instance.Network().AddHost(ctx, apiPort)
	if err != nil {
		return err
	}
	p.prometheusProxyHost = proxyHost

	return nil
}

// GetMetric returns the first matching metric from the prometheus instance
// Example:
// GetMetric(MetricFilter{MetricName: "node_start_ts", Labels: map[string]string{"instance_name": "12D3KooWNHuCns..."}})
func (p *Prometheus) GetMetric(filter MetricFilter) (float64, error) {

	labels := []string{}
	for k, v := range filter.Labels {
		labels = append(labels, fmt.Sprintf(`%s="%s"`, k, v))
	}
	query := fmt.Sprintf("%s{%s}", filter.MetricName, strings.Join(labels, ","))

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=%s", p.prometheusProxyHost, query))
	if err != nil {
		return 0, ErrFailedToFetchPrometheusData.Wrap(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, ErrFailedToFetchPrometheusData.Wrap(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, ErrFailedToParsePrometheusMetrics.Wrap(err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return 0, ErrMetricNotFound.WithParams(filter.MetricName)
	}

	results, ok := data["result"].([]interface{})
	if !ok || len(results) == 0 {
		return 0, ErrMetricNotFound.WithParams(filter.MetricName)
	}

	metric, ok := results[0].(map[string]interface{})
	if !ok {
		return 0, ErrMetricNotFound.WithParams(filter.MetricName)
	}

	valueArray, ok := metric["value"].([]interface{})
	if !ok || len(valueArray) < 2 {
		return 0, ErrMetricNotFound.WithParams(filter.MetricName)
	}

	// valueArray[0] is the timestamp
	// valueArray[1] is the metric value
	metricValueStr, ok := valueArray[1].(string)
	if !ok {
		return 0, ErrFailedToParsePrometheusMetrics.
			Wrap(fmt.Errorf("value is not a string is %T", valueArray[1]))
	}

	metricValue, err := strconv.ParseFloat(metricValueStr, 64)
	if err != nil {
		return 0, ErrFailedToParsePrometheusMetrics.Wrap(err)
	}

	return metricValue, nil
}

func (p *Prometheus) generateConfig() string {
	conf := fmt.Sprintf(`
global:
  scrape_interval: %s

scrape_configs:
`, defaultScrapeInterval.String())

	for _, j := range p.scrapeJobs {
		conf += fmt.Sprintf(`
  - job_name: %s
    scrape_interval: %s
    static_configs:
      - targets: [%s]
`, j.Name, j.Interval.String(), j.Target)
	}

	return conf
}
