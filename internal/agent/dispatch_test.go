package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/valpere/depl-orch/internal/pipeline"
)

func TestStageRecovery_RoutesToCorrectFixer(t *testing.T) {
	build := &fakeRecoverable{}
	test := &fakeRecoverable{}
	sr := &StageRecovery{Fixers: map[string]pipeline.Recoverable{
		"build": build,
		"test":  test,
	}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	_ = sr.Recover(context.Background(), st, "build", errors.New("fail"))
	if !build.called {
		t.Error("build fixer should have been called")
	}
	if test.called {
		t.Error("test fixer should NOT have been called")
	}
}

func TestStageRecovery_PassThroughForUnknownStage(t *testing.T) {
	sr := &StageRecovery{Fixers: map[string]pipeline.Recoverable{
		"test": &fakeRecoverable{},
	}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	orig := errors.New("docker failure")
	err := sr.Recover(context.Background(), st, "docker-build", orig)
	if err != orig {
		t.Errorf("unknown stage should return original error, got %v", err)
	}
}

func TestStageRecovery_EmptyFixersPassThrough(t *testing.T) {
	sr := &StageRecovery{Fixers: map[string]pipeline.Recoverable{}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	orig := errors.New("fail")
	err := sr.Recover(context.Background(), st, "test", orig)
	if err != orig {
		t.Errorf("empty fixers should return original error, got %v", err)
	}
}
