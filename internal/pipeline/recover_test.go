package pipeline

import (
	"context"
	"errors"
	"testing"
)

type fakeRecoverer struct {
	called int
	ret    error
	onCall func()
}

func (r *fakeRecoverer) Recover(_ context.Context, _ *State, _ string, _ error) error {
	r.called++
	if r.onCall != nil {
		r.onCall()
	}
	return r.ret
}

func TestRunner_RecoversAndRetries(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	rec := &fakeRecoverer{onCall: func() { fc.failOn = "" }} // recovery makes the test pass
	r := &Runner{Stages: DefaultStages(fc), Recoverer: rec, MaxRetries: 1}
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if rec.called != 1 {
		t.Errorf("recoverer called %d times, want 1", rec.called)
	}
	var sawPush bool
	for _, c := range fc.calls {
		if c == "docker push docker.io/pereval/app:test" {
			sawPush = true
		}
	}
	if !sawPush {
		t.Errorf("pipeline did not continue past the recovered stage: %v", fc.calls)
	}
}

func TestRunner_UnrecoverableFails(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	rec := &fakeRecoverer{ret: errors.New("cannot fix")}
	r := &Runner{Stages: DefaultStages(fc), Recoverer: rec, MaxRetries: 2}
	err := r.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected unrecovered failure")
	}
	if rec.called != 1 {
		t.Errorf("recoverer called %d times, want 1 (stop on its error)", rec.called)
	}
}

func TestRunner_NoRecoveryWhenMaxRetriesZero(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	rec := &fakeRecoverer{}
	r := &Runner{Stages: DefaultStages(fc), Recoverer: rec, MaxRetries: 0}
	if err := r.Run(context.Background(), newState()); err == nil {
		t.Fatal("expected failure with MaxRetries=0")
	}
	if rec.called != 0 {
		t.Errorf("recoverer must not be called when MaxRetries=0, got %d", rec.called)
	}
}
