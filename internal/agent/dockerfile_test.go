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

// minimalRepo creates a temp git repo with a single committed file.
func minimalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
	git("commit", "-m", "init")
	return dir
}

func TestGenerateDockerfile_WritesDockerfile(t *testing.T) {
	dir := minimalRepo(t)
	dockerfileContent := "FROM golang:1.26-alpine AS builder\nWORKDIR /app\nCOPY . .\nRUN go build -o app .\n\nFROM alpine\nCOPY --from=builder /app/app /app\nENTRYPOINT [\"/app\"]\n"
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall("Dockerfile", dockerfileContent),
		schema.AssistantMessage("Generated Dockerfile.", nil),
	}}
	st := &pipeline.State{
		WorkDir:    dir,
		Dockerfile: "Dockerfile",
		ImageRef:   "test:latest",
		Outputs:    map[string][]byte{"docker-build": []byte("unable to prepare context: path not found")},
	}
	if err := (&GenerateDockerfile{Model: m}).Recover(context.Background(), st, "docker-build", nil); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("Dockerfile not found after recovery: %v", err)
	}
	if len(data) == 0 {
		t.Error("Dockerfile should not be empty")
	}
}

func TestGenerateDockerfile_PassesThroughWrongStage(t *testing.T) {
	gd := &GenerateDockerfile{Model: &fakeModel{}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	origErr := context.Canceled
	err := gd.Recover(context.Background(), st, "build", origErr)
	if err != origErr {
		t.Errorf("wrong stage should return original error, got %v", err)
	}
}

func TestGenerateDockerfile_RollsBackWhenNoDockerfile(t *testing.T) {
	dir := minimalRepo(t)
	// Model takes no action → Dockerfile never written → rollback.
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage("Nothing to fix.", nil),
	}}
	st := &pipeline.State{
		WorkDir:    dir,
		Dockerfile: "Dockerfile",
		Outputs:    map[string][]byte{"docker-build": []byte("fail")},
	}
	err := (&GenerateDockerfile{Model: m}).Recover(context.Background(), st, "docker-build", nil)
	if err == nil {
		t.Error("expected error when Dockerfile not created")
	}
}

func TestGenerateDockerfile_RollsBackOnModelError(t *testing.T) {
	dir := minimalRepo(t)
	m := &fakeErrorModel{err: context.DeadlineExceeded}
	st := &pipeline.State{
		WorkDir:    dir,
		Dockerfile: "Dockerfile",
		Outputs:    map[string][]byte{"docker-build": []byte("fail")},
	}
	err := (&GenerateDockerfile{Model: m}).Recover(context.Background(), st, "docker-build", nil)
	if err == nil {
		t.Error("expected error on model failure")
	}
}
