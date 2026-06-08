package pipeline

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthCheckStage_Name(t *testing.T) {
	s := HealthCheckStage("http://example.com", time.Second, time.Second)
	if s.Name() != "health-check" {
		t.Errorf("Name() = %q, want \"health-check\"", s.Name())
	}
}

func TestHealthCheckStage_NoopWhenURLEmpty(t *testing.T) {
	s := HealthCheckStage("", 5*time.Second, time.Second)
	if err := s.Run(context.Background(), newState()); err != nil {
		t.Errorf("empty URL should be a no-op, got: %v", err)
	}
}

func TestHealthCheckStage_SuccessOn2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := HealthCheckStage(srv.URL, 5*time.Second, 50*time.Millisecond)
	if err := s.Run(context.Background(), newState()); err != nil {
		t.Errorf("expected success on 200, got: %v", err)
	}
}

func TestHealthCheckStage_RetriesUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable) // first 2 fail
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := HealthCheckStage(srv.URL, 5*time.Second, 50*time.Millisecond)
	if err := s.Run(context.Background(), newState()); err != nil {
		t.Errorf("expected eventual success, got: %v", err)
	}
	if calls.Load() < 3 {
		t.Errorf("expected at least 3 calls, got %d", calls.Load())
	}
}

func TestHealthCheckStage_TimesOutOnPersistentFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := HealthCheckStage(srv.URL, 150*time.Millisecond, 40*time.Millisecond)
	err := s.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHealthCheckStage_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	s := HealthCheckStage(srv.URL, 10*time.Second, 30*time.Millisecond)
	err := s.Run(ctx, newState())
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
}

func TestHealthCheckStage_ConnectionErrorTimesOut(t *testing.T) {
	// Port 1 is reserved and never listens — immediate connection refused.
	s := HealthCheckStage("http://127.0.0.1:1/health", 150*time.Millisecond, 40*time.Millisecond)
	err := s.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected timeout error on unreachable host, got nil")
	}
}

func TestHealthCheckStage_IntervalClampedWhenZero(t *testing.T) {
	// Zero interval must be clamped to 2s — verify the stage succeeds without spin-looping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := HealthCheckStage(srv.URL, 5*time.Second, 0)
	hcs := s.(healthCheckStage)
	if hcs.interval < 2*time.Second {
		t.Errorf("interval not clamped: got %v, want ≥2s", hcs.interval)
	}
	if err := s.Run(context.Background(), newState()); err != nil {
		t.Errorf("expected success with zero interval, got: %v", err)
	}
}

func TestHealthCheckStage_4xxIsNotSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := HealthCheckStage(srv.URL, 150*time.Millisecond, 40*time.Millisecond)
	err := s.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected timeout error for 4xx response, got nil")
	}
}

// funcStage is a test helper that wraps an arbitrary function as a Stage.
type funcStage struct {
	name string
	fn   func(context.Context, *State) error
}

func (f funcStage) Name() string                            { return f.name }
func (f funcStage) Run(ctx context.Context, st *State) error { return f.fn(ctx, st) }

func TestRunner_HealthCheckRunsAfterDeploy(t *testing.T) {
	var deployRan bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deploy := funcStage{name: "deploy", fn: func(_ context.Context, _ *State) error {
		deployRan = true
		return nil
	}}
	r := &Runner{
		Stages: []Stage{deploy, HealthCheckStage(srv.URL, 2*time.Second, 50*time.Millisecond)},
		Log:    slog.Default(),
	}
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !deployRan {
		t.Error("deploy stage was not run before health-check")
	}
}

func TestRunner_HealthCheckFailureFailsPipeline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := &Runner{
		Stages: []Stage{HealthCheckStage(srv.URL, 100*time.Millisecond, 30*time.Millisecond)},
		Log:    slog.Default(),
	}
	err := r.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected pipeline failure when health-check times out")
	}
}
