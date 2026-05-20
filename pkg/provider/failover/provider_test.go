package failover_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/provider/failover"
)

// errProvider is a Provider whose Stream always returns (nil, error).
type errProvider struct{ err error }

func (e *errProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Event, error) {
	return nil, e.err
}

func collectEvents(t *testing.T, ch <-chan provider.Event) []provider.Event {
	t.Helper()
	var events []provider.Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestFailover_FirstSucceeds_NoSwitch(t *testing.T) {
	called2 := false
	p2 := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "from-p2"},
		{Kind: provider.EventMessageStop},
	})
	_ = p2
	// Wrap p2 to detect if it was called.
	var p2WasCalled bool
	p2spy := &spyProvider{inner: p2, called: &p2WasCalled}

	p1 := fake.New([]provider.Event{
		{Kind: provider.EventStart},
		{Kind: provider.EventTextDelta, Text: "hello"},
		{Kind: provider.EventMessageStop},
	})

	fp := failover.New([]provider.Provider{p1, p2spy}, []string{"p1", "p2"})
	ch, err := fp.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(t, ch)
	if p2WasCalled || called2 {
		t.Error("p2 should NOT have been called")
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events from p1, got %d", len(events))
	}
	if events[1].Text != "hello" {
		t.Errorf("expected text 'hello', got %q", events[1].Text)
	}
}

func TestFailover_FirstFails_SecondSucceeds(t *testing.T) {
	// p1 emits EventError immediately (no committed events).
	p1 := fake.New([]provider.Event{
		{Kind: provider.EventError, Err: errors.New("p1 down")},
	})
	// p2 emits clean stream.
	p2 := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "from-p2"},
		{Kind: provider.EventMessageStop},
	})

	fp := failover.New([]provider.Provider{p1, p2}, []string{"p1", "p2"})
	ch, err := fp.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(t, ch)
	if len(events) != 2 {
		t.Fatalf("expected 2 events from p2, got %d", len(events))
	}
	if events[0].Text != "from-p2" {
		t.Errorf("expected text 'from-p2', got %q", events[0].Text)
	}
	if events[1].Kind != provider.EventMessageStop {
		t.Errorf("expected EventMessageStop, got %v", events[1].Kind)
	}
}

func TestFailover_FirstFailsAfterText_NoSwitch(t *testing.T) {
	var p2WasCalled bool
	p2inner := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "p2-text"},
	})
	p2spy := &spyProvider{inner: p2inner, called: &p2WasCalled}

	// p1 emits text_delta then EventError (committed=true).
	p1 := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "partial"},
		{Kind: provider.EventError, Err: errors.New("mid-stream error")},
	})

	fp := failover.New([]provider.Provider{p1, p2spy}, []string{"p1", "p2"})
	ch, err := fp.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(t, ch)

	if p2WasCalled {
		t.Error("p2 should NOT have been called (committed event was already emitted)")
	}
	// We expect: text_delta("partial") + EventError
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(events), events)
	}
	if events[0].Kind != provider.EventTextDelta {
		t.Errorf("expected text_delta first, got %v", events[0].Kind)
	}
	if events[1].Kind != provider.EventError {
		t.Errorf("expected EventError second, got %v", events[1].Kind)
	}
}

func TestFailover_AllFail(t *testing.T) {
	p1 := fake.New([]provider.Event{
		{Kind: provider.EventError, Err: errors.New("p1 error")},
	})
	p2 := fake.New([]provider.Event{
		{Kind: provider.EventError, Err: errors.New("p2 error")},
	})

	fp := failover.New([]provider.Provider{p1, p2}, []string{"p1", "p2"})
	ch, err := fp.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(t, ch)
	if len(events) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(events))
	}
	if events[0].Kind != provider.EventError {
		t.Errorf("expected EventError, got %v", events[0].Kind)
	}
	if events[0].Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestFailover_StartErrorRetries(t *testing.T) {
	// p1's Stream() returns (nil, error) directly.
	p1 := &errProvider{err: errors.New("connection refused")}
	// p2 succeeds.
	p2 := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "fallback"},
		{Kind: provider.EventMessageStop},
	})

	fp := failover.New([]provider.Provider{p1, p2}, []string{"p1", "p2"})
	ch, err := fp.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(t, ch)
	if len(events) != 2 {
		t.Fatalf("expected 2 events from p2, got %d", len(events))
	}
	if events[0].Text != "fallback" {
		t.Errorf("expected 'fallback', got %q", events[0].Text)
	}
}

func TestFailover_NoBackends(t *testing.T) {
	fp := failover.New(nil, nil)
	_, err := fp.Stream(context.Background(), provider.Request{})
	if err == nil {
		t.Fatal("expected error for empty backends")
	}
}

// spyProvider wraps another Provider and sets *called to true on Stream.
type spyProvider struct {
	inner  provider.Provider
	called *bool
}

func (s *spyProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	*s.called = true
	return s.inner.Stream(ctx, req)
}
