// Package observability provides opt-in OTel tracing and Prometheus metrics.
// When the corresponding env vars are unset the functions are zero-overhead
// no-ops: no SDK is initialised and no ports are opened.
package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// InitOTel initialises a global TracerProvider when ANTHROGO_OTEL_ENDPOINT is
// set. Returns a shutdown func; safe to call even if init was a no-op.
func InitOTel(ctx context.Context, version string) (shutdown func(context.Context) error, err error) {
	endpoint := os.Getenv("ANTHROGO_OTEL_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("anthrogo"),
			semconv.ServiceVersion(version),
		),
	)
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// Tracer returns the package-level tracer. Always callable, never panics —
// when no provider is configured OTel returns a noop tracer.
func Tracer() trace.Tracer {
	return otel.Tracer("github.com/ricardo/anthrogo")
}
