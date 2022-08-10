# ADR #009: Telemetry

## Changelog

* 2022-07-04: Started
* 2022-07-10: Initial Draft finished
* 2022-07-11: Stylistic improvements from @renaynay
* 2022-07-14: Stylistic improvements from @liamsi
* 2022-07-15: Stylistic improvements from @rootulp and @bidon15
* 2022-07-29: Formatting fixes
* 2022-07-29: Clarify and add more info regarding Uptrace
* 2022-08-09: Cover metrics and add more info about trace

## Authors

@Wondertan @liamsi

## Glossary

* `ShrEx` - P2P Share Exchange Stack

> It's all ogre now

## Context

> Now I know why I don't like writing ADRs - because I cannot run/test them and see if they work or not.
> Hoping that quality team feedback will solve this problem!

Celestia Node needs deeper observability of each module and their components. The only integrated observability solution
we have is logging and there are two more options we need to explore from the observability triangle(tracing, metrics and logs).

There are several priorities and "why"s we need deeper observability:

* Establishing metrics/data driven engineering culture for celestia-node devs
  * Metrics and tracing allows extracting dry facts out of any software on its performance, liveness, bottlenecks,
    regressions, etc., on whole system scale, so devs can reliably respond
  * Basing on these, all the improvements can be proven with data _before_ and _after_ a change
* Roadmap adjustment after analysis of the current `ShrEx` based on real world data from:
  * Full Node reconstruction qualities
  * Data availability sampling
* Incentivized Testnet
  * Tracking participants
  * Validating done tasks with transparent evidence
  * Harvesting valuable data/insight/traces that we can analyze and improve on
* Monitoring dashboards
  * For Celestia's own DA network infrastructure, e.g. DA Network Bootstrappers
  * For the node operators
* Extend debugging arsenal for the networking heavy DA layer
  * Local development
  * Issues found with [Testground testing](https://github.com/celestiaorg/test-infra)
  * Production

This ADR is intended to outline the decisions on how to proceed with:

* Integration plan according to the priorities and the requirements
* What observability tools/dependencies to integrate
* Integration design into Celestia-Node for each observability option
* A reference document explaining "whats" and "hows" during integration in some part of the codebase
* A primer for any developer in celestia-node to quickly onboard into Telemetry

## Decisions

### Plan

#### First Priority

The first priority lies on "ShrEx" stack analysis results for Celestia project. The outcome will tell us whether
our current [Full Node reconstruction](https://github.com/celestiaorg/celestia-node/issues/602) qualities conforms to
the main network requirements, subsequently affecting the development roadmap of the celestia-node before the main
network launch. Basing on the former, the plan is focused on unblocking the reconstruction
analysis first and then proceed with steady covering of our codebase with traces for the complex codepaths as well as
metrics and dashboards for "measurables".

Fortunately, the `ShrEx` analysis can be performed with _tracing_ only(more on that in Tracing Design section below),
so the decision for the celestia-node team is to cover with traces only the _necessary_ for the current "ShrEx" stack
code as the initial response to the ADR, leaving the rest to be integrated in the background for the devs in the team
once they are free as well as for the efficient bootstrapping into the code for the new devs.

___Update:___ The `ShrEx` analysis is not the blocker nor the highest priority at the moment of writing.

#### Second Priority

The next biggest priority - incentivized testnet can be largely covered with traces as well. All participant will submit
traces from their nodes to any provided backend endpoint by us during the whole network lifespan. Later on, we will be
able to verify the data of each participant by querying historical traces. This is the feature that some backend solutions
provide, which we can use as well to extract valuable insight on how the network performs in macro view.

Even though incentivised testnet goal can be largely covered by traces in terms of observability, the metrics for this
priority are desirable, as metrics provide:

* Easily queryable time-series data
* Extensive tooling to build visualization for that data

Both, can facilitate implementation of global network observability dashboard, participant validation for the goal.

#### Third Priority

Enabling total observability of the node through metrics and traces.

### Tooling/Dependencies

#### Telemetry Golang API/Shim

The decision is to use [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) for both Metrics and Tracing:

* Minimal and golang savvy API/shim which gathers years of experience from OpenCensus/OpenMetrics and [CNCF](https://www.cncf.io/)
* Backends/exporters for all the existing timeseries monitoring DBs, e.g. Prometheus, InfluxDB. As well as tracing backends
  Jaeger, Uptrace, etc.
* <https://github.com/uptrace/opentelemetry-go-extra/tree/main/otelzap> with logging engine we use - Zap
* Provides first-class support/implementation for/of generic [OTLP](https://opentelemetry.io/docs/reference/specification/protocol/)(OpenTelemetry Protocol)
  * Generic format for any telemetry data.
  * Allows integrating otel-go once and use it with any known backend, either
    * Supporting OTLP natively
    * Or through [OTel Collector]((https://opentelemetry.io/docs/collector/))
    * Allows exporting telemetry to one endpoint only([opentelemetry-go#3055](https://github.com/open-telemetry/opentelemetry-go/issues/3055))

The discussion over this decision can be found in [celestia-node#663](https://github.com/celestiaorg/celestia-node/issues/663)
and props to @liamsi for initial kickoff and a deep dive into OpenTelemetry.

#### Tracing Backends

For tracing, there are 4 modern OSS tools that are recommended. All of them have bidirectional support with OpenTelemetry:

* [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
  * Tracing data proxy from a OTLP client to __any__ backend
  * Supports a [long list of backends](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter)
* [Uptrace](https://get.uptrace.dev/guide/#what-is-uptrace)
  * The most recent (~1 year)
  * Tight to Clickhouse DB
  * The most lightweight
  * Supports OTLP
  * OSS and can be deployed locally
  * Provides hosted solution
* [Jaeger](https://www.jaegertracing.io/)
  * The most mature
  * Started by Uber, now supported by CNCF
  * Supports multiple storages(ScyllaDB, InfluxDB, Amazon DynamoDB)
  * Supports OTLP
* [Graphana Tempo](https://grafana.com/oss/tempo/)
  * Deep integration with Graphana/Prometheus
  * Relatively new (~2 years)
  * Uses Azure, GCS, S3 or local disk for storage

Each of these backends can be used independently and depending on the use case. For us, these are main use cases:

* Local development/debugging for the private network or even public network setup
* Data collection from the Testground test runs
* Bootstrappers monitoring infrastructure
* Data collection from the incentivized testnet participants

> I am personally planning to set up the lightweight Uptrace for the local light node. Just to play around and observe
> things
>
> __UPDATE__: It turns out it is not that straightforward and adds additional overhead. See #Other-Findings

There is no strict decision on which of these backends and where to use. People taking ownership of any listed vectors
are free to use any recommended solution or any unlisted. The only backend requirement is the support of OTLP, natively
or through OTel Collector. The latter though, introduces additional infrastructure piece which adds unnecessary complexity
for node runners, and thus not recommended.

#### Metrics Backend

We only consider OSS backends.

* [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
  * Push based
  * Metrics data proxy from a OTLP client to __any__ backend
  * Supports a [long list of backends](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter)
* [Netdata](https://github.com/netdata/netdata)
  * Push based
  * Widely supported option in the Linux community
  * Written in C
  * Decade of experience optimized to bare metal
  * Perfect for local monitoring setups
  * Unfortunately, does not support OTLP
* [Uptrace](https://get.uptrace.dev/guide/#what-is-uptrace)
  * The most recent (~1 year)
  * Tight to Clickhouse DB
  * The most lightweight
  * Supports OTLP
  * OSS and can be deployed locally
  * Provides hosted solution
* Prometheus+Graphana
  * Pull based
  * No native OTLP support
    * Thought there [spec](https://github.com/open-telemetry/wg-prometheus/blob/main/specification.md) to fix this
  * Still, can be used with Otel Collector

Similarly, no strictness around backend solution with only OTLP support requirement. Natively or through OTLP exporter.

## Design

### Tracing Design

Tracing allows to see _how_ any process progresses through different modules, APIs and networks, as well as timings of
each operation and any events or errors as they occur.

A visual example of a generic tracing dashboard provided via [Uptrace](https://uptrace.dev/) backend
![tracing](img/tracing-dashboard.png)

Mainly, for `ShrEx` and reconstruction analysis we need to know if the reconstruction succeeded and the time it took for
the big block sizes(EDS >= 128). The tracing in this case would provide all the data for the whole reconstruction
operation and for each sub operation within reconstruction, e.g time spend specifically on erasure coding
> NOTE: The exact compute time is not available unless [rsmt2d#107](https://github.com/celestiaorg/rsmt2d/issues/107)
> is fixed.

#### Spans

Span represents an operation (unit of work) in a trace. They keep the time when operation _started_ and _ended_. Any
additional user defined _attributes_, operation status(success or error with an error itself) and events/logs that
may happen during the operation.

Spans also form a parent tree, meaning that each span associated to a process can have multiple sub processes or child
spans and vise-versa. Altogether, this feature allows to see the whole trace of execution of any part of the system, no
matter how complex it is. This is exactly what we need to analyze our reconstruction performance.

#### Tracing Integration Example

First, we define global pkg level tracer to create spans from within `ipld` pkg. Basically, it groups spans under
common logical namespace and extends the full name of each span.

```go
var tracer = otel.Tracer("ipld")
```

Then, we define a root span in `ipld.Retriever`:

```go
import "go.opentelemetry.io/otel"

func (r *Retriever) Retrieve(ctx context.Context, dah *da.DataAvailabilityHeader) (*rsmt2d.ExtendedDataSquare, error) {
  ctx, span := tracer.Start(ctx, "retrieve-square")
  defer span.End()

  span.SetAttributes(
    attribute.Int("size", len(dah.RowsRoots)),
    attribute.String("data_hash", hex.EncodeToString(dah.Hash())),
  )
  ...
}
```

Next, the child span in `ipld.Retriever.Reconstruct`:

```go
 ctx, span := tracer.Start(ctx, "reconstruct-square")
 defer span.End()

 // and try to repair with what we have
 err := rs.squareImported.Repair(rs.dah.RowsRoots, rs.dah.ColumnRoots, rs.codec, rs.treeFn)
 if err != nil {
  span.RecordError(err)
  return nil, err
 }
```

And lastly, the quadrant request event:

```go
    span.AddEvent("requesting quadrant", trace.WithAttributes(
        attribute.Int("axis", q.source),
        attribute.Int("x", q.x),
        attribute.Int("y", q.y),
        attribute.Int("size", len(q.roots)),
    ))
```

> The above is only examples related to our code and is a subject to change.

Here is the result of the above code sending traces visualized on Jaeger UI
![tracing](img/trace-jaeger.png)

#### Backends connection

Example for Jaeger

```go
  // Create the Jaeger exporter
  exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
  if err != nil {
      return nil, err
  }
 // then the tracer provider
    tp := tracesdk.NewTracerProvider(
        // Always be sure to batch in production.
        tracesdk.WithBatcher(exp),
        // Record information about this application in a Resource.
        tracesdk.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(service),
            attribute.String("environment", environment),
            attribute.Int64("ID", id),
        )),
    )
 // and set it globally to be used across packages
    otel.SetTracerProvider(tp)
 
 // then close it elsewhere
    tp.Shutdown(ctx)
```

We decided to use OTLP backend, and it is almost similar in terms of setup.

### Metrics Design

Metrics allows collecting time-series data from different measurable points in the application. Every measurable can
be covered via 6 instruments OpenTelemetry provides:

* ___Counter___ - synchronous instrument that measures additive non-decreasing values.
* ___UpDownCounter___ - synchronous instrument which measures additive values that increase or decrease with time.
* ___Histogram___ - synchronous instrument that produces a histogram from recorded values.
* ___CounterObserver___ - asynchronous instrument that measures additive non-decreasing values.
* ___UpDownCounterObserver___ - asynchronous instrument that measures additive values that can increase or decrease with time.
* ___GaugeObserver___ - asynchronous instrument that measures non-additive values for which sum does not produce a meaningful correct result.

#### Metrics Integration Example

Consider we want to know report current network height as a metric.

First of all, the global pkg meter has to be defined in the code related to the desired metric. In our case it is `header` pkg.

```go
var meter = global.MeterProvider().Meter("header")
```

Next, we should understand what instrument to use. On the first glance, for chain height a ___Counter___ instrument should
fit, as it is a non-decreasing value, and then we should think whether we need a sync or async version of it. For our case,
both would work and its more the question of precision we want. Sync metering would report every height change, while
the async would poke `header` pkg API periodically to get the metered data. For our example, we will go with the latter.

```go
// MonitorHead enables Otel metrics to monitor head.
func MonitorHead(store Store) {
 headC, _ := meter.AsyncInt64().Counter(
  "head",
  instrument.WithUnit(unit.Dimensionless),
  instrument.WithDescription("Subjective head of the node"),
 )

 err := meter.RegisterCallback(
  []instrument.Asynchronous{
   headC,
  },
  func(ctx context.Context) {
   head, err := store.Head(ctx)
   if err != nil {
    headC.Observe(ctx, 0, attribute.String("err", err.Error()))
    return
   }

   headC.Observe(
    ctx,
    head.Height,
    attribute.Int("square_size", len(head.DAH.RowsRoots)),
   )
  },
 )
 if err != nil {
  panic(err)
 }
}
```

The example follows a solely API-based approach without the need to integrate the metric deeper into the implementation
insides, which is nice and keeps metering decoupled from business logic. The `MonitorHead` func simply accepts the `Store`
interface and reads the information about the latest subjective header via `Head` on the node.

The API-based approach should be followed for any info level metric. Even if there is no API to get the required metric,
such API should be introduced. However, this approach is not always possible and sometimes deeper integration with code
logic is necessary to analyze performance or there are security and/or encapsulation considerations.

On the example, we can also see how any additional data can be added to the instruments via attributes or labels. It is
important to add only absolutely necessary data(more on that in Others section below) to the metrics or data which is
common over multiple time-series. In this case, we attach `square_size` of the height to know the block size of the
height. This allows us to query reported heights with some square size using backend UI. Note that there are only
powers of two(with 256 being a current limit) as unique values possible for the metric, so it won't put pressure on the
metrics backend.
  
For in Go code examples on other metric instruments consult [Uptrace Otel docs](https://uptrace.dev/opentelemetry/go-metrics.html#getting-started).

#### Backends Connection

Example for OTLP extracted from our code

```go
  opts := []otlpmetrichttp.Option{
      otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
      otlpmetrichttp.WithEndpoint(cmd.Flag(metricsEndpointFlag).Value.String()),
  }
  if ok, err := cmd.Flags().GetBool(metricsTlS); err != nil {
      panic(err)
  } else if !ok {
      opts = append(opts, otlpmetrichttp.WithInsecure())
  }
  
  exp, err := otlpmetrichttp.New(cmd.Context(), opts...)
  if err != nil {
      return err
  }
  
  pusher := controller.New(
      processor.NewFactory(
          selector.NewWithHistogramDistribution(),
          exp,
      ),
      controller.WithExporter(exp),
      controller.WithCollectPeriod(2*time.Second),
      controller.WithResource(resource.NewWithAttributes(
          semconv.SchemaURL,
          semconv.ServiceNameKey.String(fmt.Sprintf("Celestia-%s", env.NodeType.String())), 
    // Here we can add more attributes with Node information
      )),
  )
```

## Considerations

* Tracing performance
  * _Every_ method calling two more functions making network request can affect overall performance
* Metrics backend performance
  * Mainly, we should avoid sending too much data to the metrics backend through labels. Metrics is only for metrics and
not for indexing.
e.g. hash, uuid, etc not to overload the metrics backend
* Security and exported data protection
  * OTLP provides TLS support

## Other Findings

### Labels and High Cardinality

High cardinality(many different label values) issue should be always kept in mind while introducing new metrics
and labels for them. Each metric should be attached with only absolutely necessary labels and stay away from metrics
sending label __unique__ values each time, e.g. hash, uuid, etc. Doing the opposite can dramatically increase the
amount of data stored. See <https://prometheus.io/docs/practices/naming/#labels>.

### Tracing and Logging

As you will see in the examples below, tracing looks similar to logging and have almost the same semantics. In fact,
tracing is debug logging on steroids, and we can potentially consider dropping conventional _debug_ logging once we
fully cover our codebases with the tracing. Same as logging, traces can be pipe out into the stdout as prettyprinted
event log.

### Uptrace

It turns out that running only Uptrace locally Collector is PITA. It requires either:

* Using their [uptrace-go](https://github.com/uptrace/uptrace-go/blob/master/example/metrics/main.go) custom OTel wrapper
  * For some undocumented reason they decided to go with a custom wrapper while it's possible to use OTel with Uptrace
directly
* The direct usage though also requires additional frictions and does not work with defaults. Requires:
  * Token auth to send data
  * Custom URL and path
  * Maintaining config for itself and clickhouse

Overall, it is not user-friendly alternative to known projects, even thought it still does not require running Otel
Collector and absorbs both tracing and metrics.

## Further Readings

* [Uptrace tracing tools comparison](https://get.uptrace.dev/compare/distributed-tracing-tools.html)
* [Uptrace guide](https://get.uptrace.dev/guide/)
* [Uptrace OpenTelemetry Docs](https://opentelemetry.uptrace.dev/)
  * Provides simple Go API guide for metrics and traces
* [OpenTelemetry Docs](https://opentelemetry.io/docs/)
* [Prometheus Docs](https://prometheus.io/docs/introduction/overview)

## Status

Proposed
