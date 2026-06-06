//go:build integration

// Real-model fix-test recovery: a live model (default Ollama
// qwen3-coder-next:cloud) fixes a deliberately broken Go test. Gated behind the
// `integration` tag and a reachable model.
//
//	go test -tags integration -run TestFixTest ./test/e2e/...
//
// Override the model with E2E_MODEL / E2E_MODEL_BASE_URL.
package e2e

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/valpere/depl-orch/internal/agent"
	"github.com/valpere/depl-orch/internal/model"
	"github.com/valpere/depl-orch/internal/pipeline"
)

func TestFixTestEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	modelID := os.Getenv("E2E_MODEL")
	if modelID == "" {
		modelID = "qwen3-coder-next:cloud"
	}
	baseURL := os.Getenv("E2E_MODEL_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	m, err := model.New(context.Background(), model.Config{
		Backend:   "ollama",
		Model:     modelID,
		BaseURL:   baseURL,
		MaxTokens: 4096,
	})
	if err != nil {
		t.Fatalf("model.New: %v", err)
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module calc\n\ngo 1.26\n")
	writeFile(t, dir, "add.go", "package calc\n\nfunc Add(a, b int) int {\n\treturn a - b // bug\n}\n")
	writeFile(t, dir, "add_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatalf(\"Add(2,3)=%d, want 5\", Add(2, 3))\n\t}\n}\n")
	gitInit(t, dir)

	st := pipeline.NewState(dir, "Dockerfile", "img:tag", false)
	st.Outputs["test"] = []byte("--- FAIL: TestAdd\n    add_test.go:6: Add(2,3)=-1, want 5\nFAIL")

	f := &agent.FixTest{Model: m, MaxIterations: 6}
	if err := f.Recover(context.Background(), st, "test", errors.New("test failed")); err != nil {
		t.Fatalf("real fix-test did not recover: %v", err)
	}
	// Recover only returns nil after it verified `go test ./...` passes.
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "broken"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
