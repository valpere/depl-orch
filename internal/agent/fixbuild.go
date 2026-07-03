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

// FixBuild is a bounded, reversible agentic recovery step for a failing build
// stage. It implements pipeline.Recoverable.
type FixBuild struct {
	Model         model.ToolCallingChatModel
	MaxIterations int
	Log           *slog.Logger
}

func (f *FixBuild) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	if stage != "build" {
		return stageErr
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

	failing := string(st.Outputs["build"])
	historyPrompt := buildHistoryPrompt(root, "build")
	msgs := []*schema.Message{
		schema.SystemMessage(fixBuildSystemPrompt + historyPrompt),
		schema.UserMessage("`go build ./...` failed:\n\n" + failing + "\n\nFix the source so the package compiles. Do not modify test files."),
	}

	log.Info("fix-build: starting recovery loop", "maxIterations", f.maxIter())
	if _, err := runToolLoop(ctx, f.Model, tools, msgs, f.maxIter()); err != nil && !errors.Is(err, errLoopExhausted) {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-build loop: %w (rolled back)", err)
	}

	if out, err := pipeline.DefaultCommander.Run(ctx, root, "go", "build", "./..."); err != nil {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-build did not fix compile errors: %w (rolled back)\n%s", err, tailString(out, 1000))
	}

	if diffOut, err := pipeline.DefaultCommander.Run(ctx, root, "git", "diff", snapshot); err == nil && len(diffOut) > 0 {
		appendHistory(root, HistoryItem{Stage: "build", Error: failing, Diff: string(diffOut), Timestamp: time.Now()})
	}

	f.writeRationale(root, failing)
	log.Info("fix-build: package builds after fix")
	return nil
}

func (f *FixBuild) maxIter() int {
	if f.MaxIterations > 0 {
		return f.MaxIterations
	}
	return defaultMaxIterations
}

func (f *FixBuild) writeRationale(root, failing string) {
	dir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	body := fmt.Sprintf("# fix-build recovery\n\nApplied at %s.\n\n## Original failure\n\n```\n%s\n```\n\nThe agent edited source (tests untouched) until `go build ./...` passed.\n",
		time.Now().Format(time.RFC3339), failing)
	_ = os.WriteFile(filepath.Join(dir, "fix-build.md"), []byte(body), 0o644)
}

const fixBuildSystemPrompt = `You are a senior Go engineer. A package fails to compile.
Fix the SOURCE code so that ` + "`go build ./...`" + ` succeeds.

Rules:
- NEVER modify test files (*_test.go).
- Edit only non-test source files.
- Make the minimal change that fixes the actual compile error.
- Use read_file to inspect files and write_file to apply fixes (paths are
  repository-relative).
- When the fix is complete, reply briefly without calling any more tools.`
