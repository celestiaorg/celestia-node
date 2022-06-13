package telemetry

import (
	"context"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.uber.org/fx"
)

var tp *trace.TracerProvider
var tracingFile *os.File

func newExporter(w io.Writer) (trace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
	)
}

// newResource returns a resource describing this application.
func newResource() *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(instrumentationName),
		),
	)
	return r
}

func tracing() fx.Option {
	return fx.Invoke(
		func(lf fx.Lifecycle) {
			lf.Append(fx.Hook{
				OnStart: func(ctx context.Context) (err error) {
					tracingFile, err = os.Create("traces.txt")
					if err != nil {
						return err
					}

					exp, err := newExporter(tracingFile)
					if err != nil {
						return err
					}

					tp = trace.NewTracerProvider(
						trace.WithBatcher(exp),
						trace.WithResource(newResource()),
					)
					otel.SetTracerProvider(tp)

					return nil
				},
				OnStop: func(ctx context.Context) error {
					defer tracingFile.Close()
					if err := tp.Shutdown(ctx); err != nil {
						return err
					}
					return nil
				},
			})
		},
	)
}
