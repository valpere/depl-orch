package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/valpere/depl-orch/internal/pipeline"
)

// repoWithBrokenWorkflow creates a temp git repo with an invalid YAML workflow file.
func repoWithBrokenWorkflow(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately broken YAML (unbalanced bracket).
	write(".github/workflows/ci.yml", "name: CI\non: [push\njobs:\n  build:\n    runs-on: ubuntu-latest\n")
	write("main.go", "package main\n")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t.com")
	git("config", "user.name", "t")
	git("add", ".")
	git("commit", "-m", "broken workflow")
	return dir
}

const validWorkflowYAML = `name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`

func TestFixWorkflow_FixesBrokenYAML(t *testing.T) {
	dir := repoWithBrokenWorkflow(t)
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall(".github/workflows/ci.yml", validWorkflowYAML),
		schema.AssistantMessage("Fixed YAML.", nil),
	}}
	st := &pipeline.State{
		WorkDir: dir,
		Outputs: map[string][]byte{"workflow": []byte("ci.yml: yaml: line 2: bad bracket")},
	}
	if err := (&FixWorkflow{Model: m}).Recover(context.Background(), st, "workflow", nil); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}
}

func TestFixWorkflow_PassesThroughWrongStage(t *testing.T) {
	fw := &FixWorkflow{Model: &fakeModel{}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	origErr := context.Canceled
	err := fw.Recover(context.Background(), st, "build", origErr)
	if err != origErr {
		t.Errorf("wrong stage should return original error, got %v", err)
	}
}

func TestFixWorkflow_RollsBackWhenYAMLStillInvalid(t *testing.T) {
	dir := repoWithBrokenWorkflow(t)
	// Model writes still-broken YAML.
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall(".github/workflows/ci.yml", "name: CI\non: [push\n"),
		schema.AssistantMessage("Done.", nil),
	}}
	st := &pipeline.State{
		WorkDir: dir,
		Outputs: map[string][]byte{"workflow": []byte("yaml: bad bracket")},
	}
	err := (&FixWorkflow{Model: m}).Recover(context.Background(), st, "workflow", nil)
	if err == nil {
		t.Error("expected error when YAML still invalid after agent run")
	}
}

func TestFixWorkflow_RollsBackOnModelError(t *testing.T) {
	dir := repoWithBrokenWorkflow(t)
	m := &fakeErrorModel{err: context.DeadlineExceeded}
	st := &pipeline.State{
		WorkDir: dir,
		Outputs: map[string][]byte{"workflow": []byte("yaml: bad bracket")},
	}
	err := (&FixWorkflow{Model: m}).Recover(context.Background(), st, "workflow", nil)
	if err == nil {
		t.Error("expected error on model failure")
	}
}
