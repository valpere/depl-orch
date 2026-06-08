package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeObserver records every ObserveStageAttempt call for assertions.
type fakeObserver struct {
	calls []observeCall
}

type observeCall struct {
	stage string
	err   error
}

func (o *fakeObserver) ObserveStageAttempt(stage string, _ time.Duration, err error) {
	o.calls = append(o.calls, observeCall{stage, err})
}

func TestRunner_ObserverCalledOnSuccess(t *testing.T) {
	fc := &fakeCommander{}
	obs := &fakeObserver{}
	r := &Runner{Stages: DefaultStages(fc), Observer: obs}
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 4 stages, each successful — observer must receive 4 ok calls.
	if len(obs.calls) != 4 {
		t.Fatalf("expected 4 observer calls, got %d: %+v", len(obs.calls), obs.calls)
	}
	for _, c := range obs.calls {
		if c.err != nil {
			t.Errorf("observer saw err=%v on stage %q, want nil", c.err, c.stage)
		}
	}
}

func TestRunner_ObserverCalledOnFailure(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	obs := &fakeObserver{}
	r := &Runner{Stages: DefaultStages(fc), Observer: obs}
	_ = r.Run(context.Background(), newState())

	// build (ok) + test (failed) = 2 calls
	if len(obs.calls) != 2 {
		t.Fatalf("expected 2 observer calls, got %d: %+v", len(obs.calls), obs.calls)
	}
	if obs.calls[0].err != nil {
		t.Errorf("build call should be ok, got err=%v", obs.calls[0].err)
	}
	if obs.calls[1].err == nil {
		t.Errorf("test call should have non-nil err")
	}
	if obs.calls[1].stage != "test" {
		t.Errorf("failing stage should be %q, got %q", "test", obs.calls[1].stage)
	}
}

func TestRunner_ObserverCalledEachRetry(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	rec := &fakeRecoverer{onCall: func() { fc.failOn = "" }} // recovery fixes the failure
	obs := &fakeObserver{}
	r := &Runner{Stages: DefaultStages(fc), Recoverer: rec, MaxRetries: 1, Observer: obs}
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}

	// build(ok), test(fail), test(ok after recovery), docker-build(ok), docker-push(ok) = 5
	if len(obs.calls) != 5 {
		t.Fatalf("expected 5 observer calls (incl. 1 retry), got %d: %+v", len(obs.calls), obs.calls)
	}
	// Second call is the failed test attempt
	if obs.calls[1].stage != "test" || obs.calls[1].err == nil {
		t.Errorf("call[1] should be failed test, got %+v", obs.calls[1])
	}
	// Third call is the successful retry
	if obs.calls[2].stage != "test" || obs.calls[2].err != nil {
		t.Errorf("call[2] should be recovered test, got %+v", obs.calls[2])
	}
}

func TestRunner_NilObserverDoesNotPanic(t *testing.T) {
	fc := &fakeCommander{}
	r := &Runner{Stages: DefaultStages(fc)} // Observer is nil — M1 behaviour
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("nil Observer must not panic: %v", err)
	}
}

func TestRunner_NilObserverOnFailureDoesNotPanic(t *testing.T) {
	fc := &fakeCommander{failOn: "go build"}
	r := &Runner{Stages: DefaultStages(fc)} // Observer is nil
	err := r.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected error when stage fails")
	}
	if !errors.Is(err, err) { // trivially true — just confirm we got here without panic
		t.Error("unreachable")
	}
}
