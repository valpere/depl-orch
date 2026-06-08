package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

// rollbackRecorder is a Deployer stub that records Rollback calls.
type rollbackRecorder struct {
	rolled  bool
	rollErr error
}

func (r *rollbackRecorder) Name() string { return "fake" }

func (r *rollbackRecorder) Deploy(_ context.Context, _ *State) error { return nil }

func (r *rollbackRecorder) Rollback(_ context.Context, _ *State) error {
	r.rolled = true
	return r.rollErr
}

func TestWithRollback_PreservesName(t *testing.T) {
	inner := funcStage{name: "health-check", fn: func(_ context.Context, _ *State) error { return nil }}
	s := WithRollback(inner, &rollbackRecorder{})
	if s.Name() != "health-check" {
		t.Errorf("Name() = %q, want \"health-check\"", s.Name())
	}
}

func TestWithRollback_PassthroughOnSuccess(t *testing.T) {
	rec := &rollbackRecorder{}
	s := WithRollback(
		funcStage{name: "hc", fn: func(_ context.Context, _ *State) error { return nil }},
		rec,
	)
	if err := s.Run(context.Background(), newState()); err != nil {
		t.Fatalf("expected nil on success, got: %v", err)
	}
	if rec.rolled {
		t.Error("Rollback must not be called when inner stage succeeds")
	}
}

func TestWithRollback_CallsRollbackOnFailure(t *testing.T) {
	innerErr := errors.New("health-check timed out")
	rec := &rollbackRecorder{}
	s := WithRollback(
		funcStage{name: "hc", fn: func(_ context.Context, _ *State) error { return innerErr }},
		rec,
	)
	err := s.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, innerErr) {
		t.Errorf("expected original error to be wrapped, got: %v", err)
	}
	if !rec.rolled {
		t.Error("expected Rollback to be called on failure")
	}
}

func TestWithRollback_BothErrorsReportedWhenRollbackFails(t *testing.T) {
	innerErr := errors.New("health timeout")
	rbErr := errors.New("helm rollback failed")
	rec := &rollbackRecorder{rollErr: rbErr}
	s := WithRollback(
		funcStage{name: "hc", fn: func(_ context.Context, _ *State) error { return innerErr }},
		rec,
	)
	err := s.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, innerErr) {
		t.Errorf("original error not wrapped: %v", err)
	}
	if !errors.Is(err, rbErr) {
		t.Errorf("rollback error not wrapped (errors.Is): %v", err)
	}
}

func TestWithRollback_FreshContextForRollback(t *testing.T) {
	// Simulate a cancelled pipeline context. Rollback must still execute with
	// a fresh, non-cancelled context — check properties at call time, before
	// the deferred cancel() fires.
	pipelineCtx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Run is called

	var (
		rollbackCalledWithErr   error
		rollbackHasFutureDeadline bool
		called                  bool
	)
	rec := &rollbackRecorder{}
	custom := &contextCapturingDeployer{rec: rec, capture: func(c context.Context) {
		called = true
		rollbackCalledWithErr = c.Err() // must be nil at call time
		dl, ok := c.Deadline()
		rollbackHasFutureDeadline = ok && time.Until(dl) > 0
	}}
	s := WithRollback(
		funcStage{name: "hc", fn: func(_ context.Context, _ *State) error { return errors.New("fail") }},
		custom,
	)
	_ = s.Run(pipelineCtx, newState())

	if !called {
		t.Fatal("Rollback was not called")
	}
	if rollbackCalledWithErr != nil {
		t.Errorf("rollback context was already cancelled at call time: %v", rollbackCalledWithErr)
	}
	if !rollbackHasFutureDeadline {
		t.Error("rollback context must have a future deadline (2-minute timeout)")
	}
}

// contextCapturingDeployer wraps rollbackRecorder and captures the context
// passed to Rollback for inspection.
type contextCapturingDeployer struct {
	rec     *rollbackRecorder
	capture func(context.Context)
}

func (d *contextCapturingDeployer) Name() string { return d.rec.Name() }

func (d *contextCapturingDeployer) Deploy(ctx context.Context, st *State) error {
	return d.rec.Deploy(ctx, st)
}

func (d *contextCapturingDeployer) Rollback(ctx context.Context, st *State) error {
	d.capture(ctx)
	return d.rec.Rollback(ctx, st)
}
