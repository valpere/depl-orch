package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// fakeModel is a scripted ToolCallingChatModel: each Generate call returns the
// next canned response. No network, no real model.
type fakeModel struct {
	responses []*schema.Message
	i         int
}

func (m *fakeModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.i >= len(m.responses) {
		return schema.AssistantMessage("done", nil), nil
	}
	r := m.responses[m.i]
	m.i++
	return r, nil
}

func (m *fakeModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented in fake")
}

func (m *fakeModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// writeFileCall builds an assistant turn that calls write_file with the given
// repo-relative path and content.
func writeFileCall(path, content string) *schema.Message {
	args := `{"path":` + jsonString(path) + `,"content":` + jsonString(content) + `}`
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:       "call-1",
			Type:     "function",
			Function: schema.FunctionCall{Name: "write_file", Arguments: args},
		}},
	}
}

func jsonString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}

// brokenRepo creates a temp Go module + git repo with a deliberately broken
// Add (returns a-b) and a test expecting a+b, all committed. Returns the dir.
func brokenRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module calc\n\ngo 1.26\n")
	write("add.go", "package calc\n\nfunc Add(a, b int) int {\n\treturn a - b // bug\n}\n")
	write("add_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatalf(\"Add(2,3)=%d, want 5\", Add(2, 3))\n\t}\n}\n")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.email=t@t", "-c", "user.name=t", "add", "-A")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "broken")
	return dir
}

func stateFor(dir string) *pipeline.State {
	st := pipeline.NewState(dir, "Dockerfile", "img:tag", false)
	st.Outputs["test"] = []byte("--- FAIL: TestAdd\n    add_test.go:6: Add(2,3)=-1, want 5\nFAIL")
	return st
}

func TestFixTest_FixesAndKeeps(t *testing.T) {
	dir := brokenRepo(t)
	fixed := "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall("add.go", fixed),
		schema.AssistantMessage("fixed the operator", nil),
	}}

	f := &FixTest{Model: m}
	if err := f.Recover(context.Background(), stateFor(dir), "test", errors.New("test failed")); err != nil {
		t.Fatalf("Recover returned error: %v", err)
	}

	// add.go now has the fix and tests pass.
	got, _ := os.ReadFile(filepath.Join(dir, "add.go"))
	if !strings.Contains(string(got), "a + b") {
		t.Errorf("add.go not fixed: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "fix-test.md")); err != nil {
		t.Errorf("rationale not written: %v", err)
	}
}

func TestFixTest_RollsBackWhenStillFailing(t *testing.T) {
	dir := brokenRepo(t)
	stillBad := "package calc\n\nfunc Add(a, b int) int {\n\treturn a * b // still wrong\n}\n"
	m := &fakeModel{responses: []*schema.Message{
		writeFileCall("add.go", stillBad),
		schema.AssistantMessage("tried", nil),
	}}

	f := &FixTest{Model: m}
	err := f.Recover(context.Background(), stateFor(dir), "test", errors.New("test failed"))
	if err == nil {
		t.Fatal("expected error for unfixable test, got nil")
	}

	// Working tree rolled back to the committed (original buggy) version.
	got, _ := os.ReadFile(filepath.Join(dir, "add.go"))
	if !strings.Contains(string(got), "a - b") {
		t.Errorf("tree not rolled back; add.go = %s", got)
	}
	if strings.Contains(string(got), "a * b") {
		t.Errorf("agent's bad edit was not rolled back: %s", got)
	}
}

func TestFixTest_IgnoresNonTestStage(t *testing.T) {
	want := errors.New("docker boom")
	f := &FixTest{Model: &fakeModel{}}
	if got := f.Recover(context.Background(), stateFor(t.TempDir()), "docker-build", want); !errors.Is(got, want) {
		t.Errorf("non-test stage should pass through the original error, got %v", got)
	}
}
