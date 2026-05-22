package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus collectors exported for instrumentation at call sites.
var (
	InFlightChats = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "anthrogo_serve_in_flight_chats",
		Help: "Number of /v1/chat requests currently in flight.",
	})
	TurnsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "anthrogo_engine_turns_total",
		Help: "Total engine turns by stop_reason.",
	}, []string{"stop_reason"})
	ToolCallsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "anthrogo_tool_calls_total",
		Help: "Total tool calls by tool name + is_error.",
	}, []string{"tool", "is_error"})
	ToolDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "anthrogo_tool_duration_seconds",
		Help:    "Tool call duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"tool"})
	ProviderStreamErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "anthrogo_provider_stream_errors_total",
		Help: "Provider stream errors by provider type + transient flag.",
	}, []string{"provider", "transient"})
	TokensIn = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "anthrogo_tokens_in_total",
		Help: "Cumulative input tokens consumed.",
	})
	TokensOut = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "anthrogo_tokens_out_total",
		Help: "Cumulative output tokens emitted.",
	})
	HookDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "anthrogo_hook_duration_seconds",
		Help: "Hook subprocess duration in seconds.",
	}, []string{"event"})
)

func init() {
	prometheus.MustRegister(
		InFlightChats,
		TurnsTotal,
		ToolCallsTotal,
		ToolDuration,
		ProviderStreamErrorsTotal,
		TokensIn,
		TokensOut,
		HookDuration,
	)
}

// StartMetricsServer starts a /metrics HTTP listener when addr is non-empty.
// Returns a shutdown func; safe to call even when addr is empty.
func StartMetricsServer(addr string) (shutdown func(context.Context) error, err error) {
	if addr == "" {
		return func(context.Context) error { return nil }, nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()
	return srv.Shutdown, nil
}
