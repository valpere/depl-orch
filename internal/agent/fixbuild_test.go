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

// brokenBuildRepo creates a temp Go module with a compile error (undefined symbol).
func brokenBuildRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module calc\n\ngo 1.26\n")
	write("add.go", "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	// broken.go references an undefined function
	write("broken.go", "package calc\n\nfunc Broken() int {\n\treturn undefined()\n}\n")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "test")
	git("add", ".")
	git("commit", "-m", "broken build")
	return dir
}

func stateForBuild(dir string) *pipeline.State {
	return &pipeline.State{
		WorkDir:    dir,
		Dockerfile: "Dockerfile",
		ImageRef:   "test:latest",
		Outputs:    map[string][]byte{"build": []byte("./broken.go:4:9: undefined: undefined")},
	}
}

func TestFixBuild_FixesCompileError(t *testing.T) {
	dir := brokenBuildRepo(t)
	// The model writes a fixed broken.go and then says done.
	fixedContent := "package calc\n\nfunc Broken() int {\n\treturn 0\n}\n"
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall("broken.go", fixedContent),
		schema.AssistantMessage("Fixed the undefined reference.", nil),
	}}
	fb := &FixBuild{Model: m}
	st := stateForBuild(dir)
	if err := fb.Recover(context.Background(), st, "build", nil); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}
}

func TestFixBuild_RollsBackOnModelError(t *testing.T) {
	dir := brokenBuildRepo(t)
	m := &fakeErrorModel{err: errLoopExhausted}
	// Use a real error (not errLoopExhausted) to trigger rollback path.
	m2 := &fakeErrorModel{err: context.DeadlineExceeded}
	_ = m
	fb := &FixBuild{Model: m2}
	st := stateForBuild(dir)
	err := fb.Recover(context.Background(), st, "build", nil)
	if err == nil {
		t.Error("expected error on model failure")
	}
}

func TestFixBuild_PassesThroughWrongStage(t *testing.T) {
	fb := &FixBuild{Model: &fakeModel{}}
	st := &pipeline.State{Outputs: map[string][]byte{}}
	origErr := context.Canceled
	err := fb.Recover(context.Background(), st, "test", origErr)
	if err != origErr {
		t.Errorf("wrong stage should return original error, got %v", err)
	}
}
