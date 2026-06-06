package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/valpere/depl-orch/internal/pipeline"
)

const defaultMaxIterations = 4

// FixTest is a bounded, reversible agentic recovery step for a failing test
// stage. It implements pipeline.Recoverable.
type FixTest struct {
	Model         model.ToolCallingChatModel
	MaxIterations int          // model loop cap (default 4)
	Log           *slog.Logger // optional
}

// Recover attempts to fix a failing `go test` by editing source, bounded and
// fully reversible. It is a no-op (returns the original error) for any stage
// other than "test". On success the tests pass and the change is kept; on
// failure or exhaustion the working tree is rolled back to its pre-attempt state.
func (f *FixTest) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	if stage != "test" {
		return stageErr // not this recoverer's job
	}
	log := f.Log
	if log == nil {
		log = slog.Default()
	}
	root := st.WorkDir

	snapshot, err := gitSnapshot(ctx, root)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	tools, err := RepoTools(root)
	if err != nil {
		return err
	}

	failing := string(st.Outputs["test"])
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("`go test ./...` failed:\n\n" + failing + "\n\nFix the source so the tests pass. Do not modify test files."),
	}

	log.Info("fix-test: starting recovery loop", "maxIterations", f.maxIter())
	// A hard loop error (model/tool call failed) is fatal → roll back. Mere budget
	// exhaustion is not: the model may have applied a working fix but kept talking,
	// so fall through to verification and let `go test` decide.
	if _, err := runToolLoop(ctx, f.Model, tools, msgs, f.maxIter()); err != nil && !errors.Is(err, errLoopExhausted) {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-test loop: %w (rolled back)", err)
	}

	// Verify the fix ourselves so the runner never re-runs a still-broken tree,
	// and an unfixable case is rolled back rather than left dirty (§2.3, §2.4).
	if out, err := pipeline.DefaultCommander.Run(ctx, root, "go", "test", "./..."); err != nil {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-test did not make tests pass: %w (rolled back)\n%s", err, tailString(out, 1000))
	}

	f.writeRationale(root, failing)
	log.Info("fix-test: tests pass after fix")
	return nil
}

func (f *FixTest) maxIter() int {
	if f.MaxIterations > 0 {
		return f.MaxIterations
	}
	return defaultMaxIterations
}

func (f *FixTest) writeRationale(root, failing string) {
	dir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	body := fmt.Sprintf("# fix-test recovery\n\nApplied at %s.\n\n## Original failure\n\n```\n%s\n```\n\nThe agent edited source (tests untouched) until `go test ./...` passed.\n",
		time.Now().Format(time.RFC3339), failing)
	_ = os.WriteFile(filepath.Join(dir, "fix-test.md"), []byte(body), 0o644)
}

func tailString(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}

const systemPrompt = `You are a senior Go engineer. A package's tests are failing.
Fix the SOURCE code so that ` + "`go test ./...`" + ` passes.

Rules:
- NEVER weaken, skip, comment out, or delete tests or their assertions.
- Edit only non-test source files (*.go that are not *_test.go).
- Make the minimal change that fixes the actual bug.
- Use read_file to inspect files and write_file to apply fixes (paths are
  repository-relative).
- When the fix is complete, reply briefly without calling any more tools.`
