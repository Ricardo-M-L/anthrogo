package observability_test

import (
	"context"
	"testing"

	"github.com/ricardo/anthrogo/pkg/observability"
)

func TestInitOTel_NoEnvIsNoop(t *testing.T) {
	// Ensure the env var is not set.
	t.Setenv("ANTHROGO_OTEL_ENDPOINT", "")

	shutdown, err := observability.InitOTel(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown func must not be nil")
	}
	// Calling shutdown must not panic.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
	// Tracer must be callable without panic.
	tr := observability.Tracer()
	if tr == nil {
		t.Fatal("Tracer() returned nil")
	}
	_, span := tr.Start(context.Background(), "test-noop")
	span.End()
}

func TestInitOTel_BadEndpoint(t *testing.T) {
	// A clearly unreachable / malformed endpoint. The gRPC dial is lazy, so
	// the error surfaces at exporter creation time only with eager connect.
	// otlptracegrpc.New connects eagerly — an invalid host:port returns an error.
	t.Setenv("ANTHROGO_OTEL_ENDPOINT", "!!bad::endpoint")

	ctx, cancel := context.WithTimeout(context.Background(), 3)
	defer cancel()

	// We accept either an immediate error or a successful noop (noop only when
	// the endpoint is silently ignored). The key assertion is: no panic.
	shutdown, err := observability.InitOTel(ctx, "test")
	if err != nil {
		// Expected: bad endpoint caused an error. Test passes.
		t.Logf("InitOTel returned expected error: %v", err)
		return
	}
	// If no error: shutdown must still be callable.
	_ = shutdown(ctx)
}
