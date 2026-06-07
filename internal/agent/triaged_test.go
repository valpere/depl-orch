package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/valpere/depl-orch/internal/pipeline"
)

type fakeRecoverable struct {
	called bool
	err    error
}

func (f *fakeRecoverable) Recover(_ context.Context, _ *pipeline.State, _ string, _ error) error {
	f.called = true
	return f.err
}

func classifierFor(class string) *Classifier {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage(`{"classification":"`+class+`","reason":"test"}`, nil),
	}}
	return &Classifier{Model: m}
}

func TestTriagedRecovery_Trivial(t *testing.T) {
	trivial := &fakeRecoverable{}
	complex := &fakeRecoverable{}
	tr := &TriagedRecovery{
		Classifier:   classifierFor("trivial"),
		TrivialFixer: trivial,
		ComplexFixer: complex,
	}
	st := &pipeline.State{Outputs: map[string][]byte{"test": []byte("fail")}}
	err := tr.Recover(context.Background(), st, "test", errors.New("fail"))
	if err != nil {
		t.Fatal(err)
	}
	if !trivial.called {
		t.Error("TrivialFixer should have been called")
	}
	if complex.called {
		t.Error("ComplexFixer should NOT have been called")
	}
}

func TestTriagedRecovery_Complex(t *testing.T) {
	trivial := &fakeRecoverable{}
	complex := &fakeRecoverable{}
	tr := &TriagedRecovery{
		Classifier:   classifierFor("complex"),
		TrivialFixer: trivial,
		ComplexFixer: complex,
	}
	st := &pipeline.State{Outputs: map[string][]byte{"test": []byte("fail")}}
	err := tr.Recover(context.Background(), st, "test", errors.New("fail"))
	if err != nil {
		t.Fatal(err)
	}
	if trivial.called {
		t.Error("TrivialFixer should NOT have been called")
	}
	if !complex.called {
		t.Error("ComplexFixer should have been called")
	}
}

func TestTriagedRecovery_NotFixable(t *testing.T) {
	trivial := &fakeRecoverable{}
	complex := &fakeRecoverable{}
	stageErr := errors.New("external service down")
	tr := &TriagedRecovery{
		Classifier:   classifierFor("not-fixable"),
		TrivialFixer: trivial,
		ComplexFixer: complex,
	}
	st := &pipeline.State{Outputs: map[string][]byte{"test": []byte("fail")}}
	err := tr.Recover(context.Background(), st, "test", stageErr)
	if err != stageErr {
		t.Errorf("got %v, want original stageErr", err)
	}
	if trivial.called || complex.called {
		t.Error("no fixer should have been called for not-fixable")
	}
}

func TestTriagedRecovery_GarbageDefaultsToComplex(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage("not json", nil),
	}}
	trivial := &fakeRecoverable{}
	complex := &fakeRecoverable{}
	tr := &TriagedRecovery{
		Classifier:   &Classifier{Model: m},
		TrivialFixer: trivial,
		ComplexFixer: complex,
	}
	st := &pipeline.State{Outputs: map[string][]byte{"test": []byte("fail")}}
	_ = tr.Recover(context.Background(), st, "test", errors.New("fail"))
	if !complex.called {
		t.Error("garbage classifier response should route to ComplexFixer")
	}
}
