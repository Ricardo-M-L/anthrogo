package observability_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/ricardo/anthrogo/pkg/observability"
)

func TestStartMetricsServer_NoAddrIsNoop(t *testing.T) {
	shutdown, err := observability.StartMetricsServer("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown func must not be nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestStartMetricsServer_ListensAndServesMetrics(t *testing.T) {
	// Pick a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	shutdown, err := observability.StartMetricsServer(addr)
	if err != nil {
		t.Fatalf("StartMetricsServer: %v", err)
	}
	t.Cleanup(func() {
		_ = shutdown(context.Background())
	})

	// Poll until the server is ready (it starts in a goroutine).
	var body string
	for range 20 {
		resp, herr := http.Get(fmt.Sprintf("http://%s/metrics", addr))
		if herr == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
	}

	if body == "" {
		t.Fatal("metrics endpoint returned no body")
	}
	if !strings.Contains(body, "anthrogo_") {
		t.Errorf("expected at least one anthrogo_* metric in output; got:\n%s", body)
	}
}
