# Observability

anthrogo ships opt-in **OTel tracing** and **Prometheus metrics**. Both are
zero-overhead when their respective env vars are unset — no SDK initialisation,
no port binding, no goroutines.

## Quick start

```bash
# Export traces to a local Jaeger/Tempo collector
export ANTHROGO_OTEL_ENDPOINT=localhost:4317

# Expose Prometheus /metrics on port 9100
export ANTHROGO_METRICS_ADDR=:9100

anthrogo serve --addr 127.0.0.1:8765
```

Then scrape metrics:

```bash
curl -sf http://localhost:9100/metrics | grep anthrogo_
```

---

## OTel tracing

### Environment variable

| Variable | Example | Description |
|---|---|---|
| `ANTHROGO_OTEL_ENDPOINT` | `localhost:4317` | OTLP/gRPC endpoint. When unset OTel is disabled. |

The connection is made **insecure** (no TLS) by default. To use a TLS-enabled
collector, point at it with the standard `OTEL_EXPORTER_OTLP_CERTIFICATE`
variable from the OTel spec alongside `ANTHROGO_OTEL_ENDPOINT`.

### Service resource

| Attribute | Value |
|---|---|
| `service.name` | `anthrogo` |
| `service.version` | binary version (e.g. `0.14.2-dev`) |

### Spans produced

| Span name | Package | Notes |
|---|---|---|
| `serve.HTTP <METHOD> <PATH>` | `internal/serve` | One per HTTP request, wraps the full handler. |
| `engine.turn` | `pkg/query` | One per `runOneAPITurnAttempt` call. |
| `provider.Stream <model>` | `pkg/query` | Child of `engine.turn`; covers the `Provider.Stream` call. |
| `tool.<name>` | `pkg/query` | One per tool execution; child of `engine.turn`. |
| `hook.<event>` | `internal/hooks` | One per `RunHook` subprocess; event = `PreToolUse`, `PostToolUse`, etc. |

### Pointing at Jaeger

```yaml
# docker-compose snippet
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "4317:4317"    # OTLP gRPC
```

```bash
ANTHROGO_OTEL_ENDPOINT=localhost:4317 anthrogo -p "hello"
# open http://localhost:16686 → search "anthrogo"
```

### Pointing at Grafana Tempo

```yaml
# tempo.yaml (minimal)
server:
  http_listen_port: 3200
distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
```

```bash
ANTHROGO_OTEL_ENDPOINT=localhost:4317 anthrogo serve --addr 127.0.0.1:8765
```

---

## Prometheus metrics

### Environment variable

| Variable | Example | Description |
|---|---|---|
| `ANTHROGO_METRICS_ADDR` | `:9100` | TCP address for the `/metrics` endpoint. When unset no server starts. |

### Metric reference

| Metric | Type | Labels | Description |
|---|---|---|---|
| `anthrogo_serve_in_flight_chats` | Gauge | — | Number of `/v1/chat` requests currently in flight. |
| `anthrogo_engine_turns_total` | Counter | `stop_reason` | Total engine turns by stop reason (`end_turn`, `tool_use`, `max_tokens`, …). |
| `anthrogo_tool_calls_total` | Counter | `tool`, `is_error` | Total tool calls by tool name and error flag. |
| `anthrogo_tool_duration_seconds` | Histogram | `tool` | Tool call duration (default Prometheus buckets). |
| `anthrogo_provider_stream_errors_total` | Counter | `provider`, `transient` | Provider stream errors by provider type and transient flag. |
| `anthrogo_tokens_in_total` | Counter | — | Cumulative input tokens consumed. |
| `anthrogo_tokens_out_total` | Counter | — | Cumulative output tokens emitted. |
| `anthrogo_hook_duration_seconds` | Histogram | `event` | Hook subprocess duration by event name. |

### Sample Prometheus scrape config

```yaml
scrape_configs:
  - job_name: anthrogo
    static_configs:
      - targets: ["localhost:9100"]
    scrape_interval: 15s
```

### Verify with curl

```bash
# Start serve with metrics enabled
ANTHROGO_METRICS_ADDR=:9100 anthrogo serve --addr 127.0.0.1:8765 &

# Wait for startup, then scrape
curl -sf http://localhost:9100/metrics | grep anthrogo_

# Example output:
# anthrogo_serve_in_flight_chats 0
# anthrogo_engine_turns_total{stop_reason="end_turn"} 3
# anthrogo_tokens_in_total 1240
# anthrogo_tokens_out_total 580
```
