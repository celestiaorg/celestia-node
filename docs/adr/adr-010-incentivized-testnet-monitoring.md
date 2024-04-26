# ADR #010: Incentivized Testnet Monitoring

## Changelog

- 2022-7-19: Started
- 2022-7-22: Add section on "How to monitor celestia-node with Grafana Cloud"
- 2022-7-26: Add section on "How to monitor celestia-node with Uptrace"
- 2022-7-29: Add section on "How to send data over HTTPS"
- 2022-8-1: Revise architecture to minimize Celestia managed components
- 2022-8-4: Add section on "Why doesn't the Celestia team host OTEL Collectors for node operators?"
- 2022-8-8: Rename section to "Which actor should run OTEL Collector(s) during the incentivized testnet?"
- 2022-8-9: Update diagrams and add "Scenario D"
- 2022-8-10: Add decision for "Which actor should run OTEL Collector(s) during the incentivized testnet?"

## Context

We're adding telemetry to celestia-node by instrumenting our codebase with metrics (see [ADR-009-telemetry](./adr-009-telemetry.md)). If the option to report metrics is enabled on celestia-node, then celestia-node will push metrics via [OTLP Exporter](https://opentelemetry.io/docs/reference/specification/protocol/exporter/) to an [OTEL Collector](https://opentelemetry.io/docs/collector/) instance.

We would like to make the metrics exported by celestia-node actionable by making them queryable in internal Grafana dashboards. We additionally want a subset of metrics to be queryable by a public incentivized testnet leaderboard frontend.

We would like to make it possible for node operators to monitor their own nodes with existing telemetry tools (e.g. Grafana and Uptrace).

This document proposes a strategy for making data available for use in internal Grafana dashboards and a public leaderboard. Additionally it describes how a node operator can deploy and configure their own OTEL Collector instance.

## Detailed Design

### Where to export data to?

Grafana can query data from [multiple data sources](https://grafana.com/docs/grafana/latest/datasources/#supported-data-sources). This document explores two of these data sources:

1. [Prometheus](https://github.com/prometheus/prometheus) is an open-source time series database written in Go. Prometheus uses the [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) query language. We can deploy Prometheus ourselves or use a hosted Prometheus provider (ex. [Google](https://cloud.google.com/stackdriver/docs/managed-prometheus), [AWS](https://aws.amazon.com/prometheus/), [Grafana](https://grafana.com/go/hosted-prometheus-monitoring/), etc.). Prometheus is pull-based which means services that would like to expose Prometheus metrics must provide an HTTP endpoint (ex. `/metrics`) that a Prometheus instance can poll (see [instrumenting a Go application for Prometheus](https://prometheus.io/docs/guides/go-application/)). Prometheus is used by [Cosmos SDK telemetry](https://docs.cosmos.network/main/learn/advanced/telemetry) and [Tendermint telemetry](https://docs.tendermint.com/main/tendermint-core/metrics.html) so one major benefit to using Prometheus is that metrics emitted by celestia-core, celestia-app, and celestia-node can share the same database.
2. [InfluxDB](https://github.com/influxdata/influxdb) is another open-source time series database written in Go. It is free to deploy InfluxDB but there is a commercial offering from [influxdata](https://www.influxdata.com/get-influxdb/) that provides clustering and on-prem deployments. InfluxDB uses the [InfluxQL](https://docs.influxdata.com/influxdb/v1.8/query_language/) query language which appears less capable at advanced queries than PromQL ([article](https://www.robustperception.io/translating-between-monitoring-languages/)). InfluxDB is push-based which means services can push metrics directly to an InfluxDB instance ([ref](https://logz.io/blog/prometheus-influxdb/#:~:text=InfluxDB%20is%20a%20push%2Dbased,and%20Prometheus%20fetches%20them%20periodically.)). See [Prometheus vs. InfluxDB](https://prometheus.io/docs/introduction/comparison/#prometheus-vs-influxdb) for a more detailed comparison.

If alternative data sources should be evaluated, please share them with us.

#### Decision

We agreed on using Prometheus at this time.

### How to export data out of OTEL Collector?

[Exporters](https://opentelemetry.io/docs/collector/configuration/#exporters) provide a way to export data from an OTEL Collector to a supported destination.

We configure OTEL Collector to export data to Prometheus like this:

```yaml
exporters:
  # Data sources: metrics
  prometheus:
    endpoint: "prometheus:8889"
    namespace: "default"
```

We must additionally enable this exporter via configuration like this:

```yaml
service:
  pipelines:
    metrics:
      exporters: [prometheus]
```

OTEL Collector support for exporting to InfluxDB is still in [beta](https://github.com/open-telemetry/opentelemetry-collector#beta=). See [InfluxDB Exporter](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter#section-readme).

### How to query data in Prometheus from Grafana?

In order to query Prometheus data from Grafana, we must add a Prometheus datasource. Steps outlined [here](https://prometheus.io/docs/visualization/grafana/#creating-a-prometheus-data-source).

### How to query data in Prometheus from incentivized testnet leaderboard?

Prometheus server exposes an HTTP API for querying metrics (see [docs](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-exemplars)).

Implementation details for the incentivized testnet leaderboard are not yet known (likely built by an external vendor). Two possible implementations are:

1. If the incentivized testnet has a dedicated backend, it can query the HTTP API above
2. If the incentivized testnet has **no** dedicated backend and the frontend queries Prometheus directly, then there exists a TypeScript library: [prometheus-query-js](https://github.com/samber/prometheus-query-js) which may be helpful.

### How can a node operator monitor their own node?

Node operators have the option of adding an additional exporter to their OTEL Collector configuration in order to export to multiple backends. This may be useful for node operators who want to configure alerting on metrics emitted by their node. A minimal guide for node operators to collect telemetry from their nodes follows:

<details>
<summary> How to monitor celestia-node with Grafana Cloud</summary>

1. [Install celestia-node](https://docs.celestia.org/developers/celestia-node)
2. Sign up for an account on [Grafana](https://grafana.com/)
3. [Install OTEL Collector](https://opentelemetry.io/docs/collector/getting-started/) on the same machine as celestia-node. If on a Linux machine follow [these steps](https://opentelemetry.io/docs/collector/getting-started/#linux-packaging=). OTEL Collector should start automatically immediately after installation.
4. Configure OTEL Collector to receive metrics from celestia-node by confirming your `/etc/otelcol/config.yaml` has the default config:

    ```yaml
    receivers:
      otlp:
        protocols:
          grpc:
          http:
    ```

    This starts the [OTLP receiver](https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/otlpreceiver/README.md) on port 4317 for gRPC and 4318 for HTTP. Celestia-node will by default emit HTTP metrics to `localhost:4318` so if you deployed OTEL Collector on the same machine as celestia-node, you can preserve the default config.
5. Configure OTEL Collector to send metrics to Prometheus. If you are using cloud-hosted Grafana, add something like the following to your `/etc/otelcol/config.yaml`:

    ```yaml
    exporters:
      prometheusremotewrite:
        endpoint: https://361398:eyJrIjoiYTNlZTFiOTc2NjA2ODJlOGY1ZGRlNGJkNWMwODRkMDY2M2U2MTE3NiIsIm4iOiJtZXRyaWNzLWtleSIsImlkIjo2MTU4ODJ9@prometheus-prod-01-eu-west-0.grafana.net/api/prom/push
    ```

6. Configure OTEL Collector to enable the `otlp` receiver and the `prometheusremotewrite` exporter. In `/etc/otelcol/config.yaml`:

    ```yaml
    service:
      pipelines:
        metrics:
          receivers: [otlp]
          exporters: [prometheusremotewrite]
    ```

    See [this article](https://grafana.com/blog/2022/05/10/how-to-collect-prometheus-metrics-with-the-opentelemetry-collector-and-grafana/) for more details. You may need to specify port 443 in the endpoint like this: `endpoint: "https://USER:PASSWORD@prometheus-blocks-prod-us-central1.grafana.net:443/api/prom/push"`

7. Restart OTEL Collector with `sudo systemctl restart otelcol`
8. Monitor that OTEL Collector started correctly with `systemctl status otelcol.service` and confirming no errors in `journalctl | grep otelcol | grep Error`
9. Start celestia-node with metrics enabled `celestia light start --core.ip https://rpc-mamaki.pops.one --metrics`
10. Verify that metrics are being displayed in Grafana.
11. [Optional] Import a [OpenTelemetry Collector Dashboard](https://grafana.com/grafana/dashboards/12553-opentelemetry-collector/) into Grafana to monitor your OTEL Collector.

</details>

<details>
<summary> How to monitor celestia-node with Uptrace</summary>

1. [Install celestia-node](https://docs.celestia.org/developers/celestia-node).
2. Create an account on [Uptrace](https://app.uptrace.dev/).
3. Create a project on Uptrace.
4. Follow [these steps](https://uptrace.dev/opentelemetry/collector.html#when-to-use-opentelemetry-collector=) to install OTEL Collector Contrib on the same host as celestia-node.
5. Configure OTEL Collector Contrib based on the [configuration](https://uptrace.dev/opentelemetry/collector.html#configuration=) section in the Uptrace docs. Ensure you selected your newly created project in the dropdown. If you'd like to collect traces and metrics, you need to add the `metrics` section under `services.pipelines`:

    ```yaml
    service:
      pipelines:
        metrics:
          receivers: [otlp]
          processors: [batch]
          exporters: [otlp]
    ```

6. Restart OTEL Collector contrib with `sudo systemctl restart otelcol-contrib`. Check that OTEL Collector Contrib is running with `sudo systemctl status otelcol-contrib` and confirm there are no errors in `sudo journalctl -u otelcol-contrib -f`. If you encounter `No journal files were found.` then reference this [StackOverflow post](https://stackoverflow.com/questions/30783134/systemd-user-journals-not-being-created/47930381#47930381).
7. Start celestia-node with metrics and traces enabled: `celestia light start --core.ip https://rpc-mamaki.pops.one --tracing --metrics`.
8. Navigate to Uptrace and create a dashboard. Confirm you can see a metric.

</details>

### Should we host a Prometheus instance ourselves or use a hosted provider?

We currently host a Prometheus instance on DigitalOcean (host mamaki-prometheus) for development. However, cloud hosted Prometheus providers take on the responsibility of running, upgrading, and scaling a Prometheus instance for us (see [oss-vs-cloud](https://grafana.com/oss-vs-cloud/). Although multiple hosted providers exist, we propose using Grafana Cloud's hosted Prometheus at this time.

### Should we host a Grafana instance ourselves or use a hosted provider?

We already host a Grafana instance on DigitalOcean (host mamaki-prometheus). We propose using Grafana Cloud's hosted Grafana at this time due to it's tight integration with Grafana Cloud Prometheus.

### Should we host separate Prometheus instances per use case? I.e. one for internal dashboards and one for public leaderboard?

The Prometheus docs state the following with regard to [Denial of Service](https://prometheus.io/docs/operating/security/#denial-of-service):

> There are some mitigations in place for excess load or expensive queries. However, if too many or too expensive queries/metrics are provided components will fall over. It is more likely that a component will be accidentally taken out by a trusted user than by malicious action.

So if we are concerned about the public leaderboard crashing the Prometheus instance that we use for internal dashboards, we may want to host two separate instances. This seems feasible by configuring OTEL Collector to export to two different Prometheus instances. This is a one way door, I suggest sticking with one instance because Grafana Cloud guarantees 99.5% uptime.

### Which actor should run OTEL Collector(s) during the incentivized testnet?

#### Scenario A: Node operators

![scenario a](./img/incentivized-testnet-monitoring-scenario-a.png)

Pros

- This deployment architecture is more representative of mainnet where full storage node operators will run their own telemetry stack to monitor their node. Exposing node operators to OTEL Collector during incentivized testnet allows them to practice this deployment architecture during incentivized testnet prior to mainnet.
- Node operators will have an "incentive" to maintain high uptime for their OTEL Collector.

Cons

- Additional operational burden for incentivized testnet participants. We can mitigate this concern by providing easy install steps and scripts.

#### Scenario B: Celestia team

![scenario b](./img/incentivized-testnet-monitoring-scenario-b.png)

Pros

- It will be easier for nodes to participate if they only have to deploy one piece of software (celestia-node) and not two (celestia-node and OTEL Collector).

Cons

- At this time, there are no cloud managed offerings for OTEL Collector. There is minimal documentation on the scale of workload an individual OTEL Collector can handle. We'd have to design and operate a highly available OTEL Collector fleet to maintain high uptime for node operators. We'd also have to mitigate DDOS attacks against our OTEL Collector endpoint (cursory investigation below).

#### Scenario C: Node operators by default and Celestia team as a fallback

![scenario c](./img/incentivized-testnet-monitoring-scenario-c.png)

Pros

- Optionality for node operators who don't want to deploy an OTEL Collector to rely on a best-effort OTEL Collector provided by Celestia team.

Cons

- This option increases the cognitive load on node operators who now have an additional decision at deployment time.
- Increased operational burden on Celestia team during incentivized testnet (and beyond).

#### Scenario D: Celestia team by default and node operators if they want

The diagram, pros, and cons are the same as scenario C.

This scenario differs from Scenario C in the docs for node deployment. The docs specify a Celestia team managed OTEL Collector endpoint by default. For node operators who want self-managed telemetry, the docs contain steps on how to setup a node operator managed agent OTEL Collector and how to proxy metrics to the Celestia team managed gateway OTEL Collector.

The docs may also contain steps on connecting the agent OTEL Collector to Prometheus and Grafana.

<!-- markdownlint-disable-next-line MD024 -->
#### Decision

We agreed on **Scenario D** at this time. Our docs will describe how to connect to the Celestia team managed OTEL Collector (i.e. `--metrics.endpoint`). Our docs will also include an advanced guide for node operators if they wish to deploy a self-managed OTEL Collector.

### Should node operators be able to configure celestia-node to export to multiple OTEL collectors?

This is not supported by [open-telemetry/opentelemetry-go#3055](https://github.com/open-telemetry/opentelemetry-go/issues/3055). This means node operators can only configure one OTLP backend for their node. If they wish to export metrics to multiple OTEL Collectors then they must route traffic through an agent OTEL Collector that they have deployed. Their agent OTEL Collector can forward metrics to any other OTEL Collector that they have access to.

### How to mitigate DDOS attacks against OTEL Collector?

- [https://medium.com/opentelemetry/securing-your-opentelemetry-collector-1a4f9fa5bd6f](https://medium.com/opentelemetry/securing-your-opentelemetry-collector-1a4f9fa5bd6f)
  - Uses an authentication server (Keycloak) to handle OAuth2 between service and remote OTEL Collector.
- [https://medium.com/@michaelericksen_12434/securing-your-opentelemetry-collector-updated-3f9884e37a09](https://medium.com/@michaelericksen_12434/securing-your-opentelemetry-collector-updated-3f9884e37a09)
  - Also uses authentication server (Keycloak). Runs services in Docker.

### How to mitigate DDOS attacks against Prometheus?

It’s possible to create an API key with the `MetricsPublisher` role on cloud hosted Prometheus. These API keys can be distributed to participants if they are expected to remote write to Prometheus.

### How to send data over HTTPS

#### OTEL Collector -> Prometheus

Uses HTTPS by default. No additional configuration needed besides copying remote endpoint from Grafana Cloud.

#### OTEL Collector -> Uptrace

Uses HTTPS by default. No additional configuration needed besides copying the data source name from Uptrace.

#### celestia-node -> OTEL Collector with public certificate

In the case where an OTEL Collector is running on a different host than celestia-node, then the OTEL Collector must be configured with a public certificate so that celestia-node can send data to it over HTTPS.

1. Ensure that celestia-node doesn't use [`WithInsecure`](https://github.com/open-telemetry/opentelemetry-go/blob/main/exporters/otlp/otlpmetric/otlpmetrichttp/options.go#L161) when constructing otlptracehttp client
1. Configure the OTEL Collector receiver to run with a TLS certificate and key. A TLS certificate can be generated with [LetsEncrypt](https://letsencrypt.org/). Example:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:
        endpoint: otel.collector.celestia.observer:4318
        tls:
          cert_file: /home/fullchain.pem
          key_file:  /home/privkey.pem
```

#### celestia-node -> OTEL Collector without public certificate

In the case where a node operator wants to send data from celestia-node to an OTEL Collector without a public certificate (e.g. node-operator managed OTEL Collector), they can issue self-signed certificate in order send data over HTTPS. Alternatively they can send data over HTTP.

1. Follow the steps at [setting up certificates](https://opentelemetry.io/docs/collector/configuration/#setting-up-certificates)
1. Configure the OTEL Collector receiver to run with this self-signed certificate. Example:

    ```yaml
    receivers:
    otlp:
        protocols:
        grpc:
        http:
            tls:
            cert_file: /home/cert.pem
            key_file: /home/cert-key.pem
    ```

1. Ensure that celestia-node runs with a TLS config that contains the Root CA created in step 1. See [sample code](https://github.com/celestiaorg/celestia-node/blob/rp/tracing-with-tls/cmd/flags_misc.go#L173-L199)

#### What are the resource requirements of OTEL collector?

Official resource requirements are not stated in the OTEL Collector docs. However, [performance benchmarks](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/performance.md#results-without-tail-based-sampling) indicate that OTEL Collector is able to handle up to 10K traces ([units unclear](https://github.com/open-telemetry/opentelemetry-collector/issues/5780)) on 1 CPU and 2GB RAM. Given [light node](https://docs.celestia.org/nodes/light-node#hardware-requirements) runs on 1 CPU and 2GB RAM, it seems feasible to run an OTEL Collector agent on the most resource constrained target hardware.

## Status

Proposed

## References

- <https://github.com/celestiaorg/celestia-node/pull/901>
- <https://github.com/celestiaorg/celestia-node/pull/907>
- <https://opentelemetry.io/docs/collector/>
- <https://celestia-team.slack.com/archives/C03QAJVLHK3/p1658169362548589>
- <https://www.notion.so/celestiaorg/Telemetry-Dashboard-d85550a3caee4004b00a2e3bf82619b1>
- <https://www.notion.so/celestiaorg/TLS-for-telemetry-6ce8e321616140a6be64ed27e99dc791>
- <https://lightstep.com/opentelemetry/collecting-and-exporting-data#opentelemetry-collector>
