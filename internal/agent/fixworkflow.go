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
	"gopkg.in/yaml.v3"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// FixWorkflow is a bounded, reversible agentic recovery step for a failing
// workflow stage. It repairs broken GitHub Actions YAML files.
// It implements pipeline.Recoverable.
type FixWorkflow struct {
	Model         model.ToolCallingChatModel
	MaxIterations int
	Log           *slog.Logger
}

func (f *FixWorkflow) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	if stage != "workflow" {
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

	failing := string(st.Outputs["workflow"])
	msgs := []*schema.Message{
		schema.SystemMessage(fixWorkflowSystemPrompt),
		schema.UserMessage("GitHub Actions workflow validation failed:\n\n" + failing + "\n\nRepair the workflow YAML files so they parse correctly."),
	}

	log.Info("fix-workflow: starting recovery loop", "maxIterations", f.maxIter())
	if _, err := runToolLoop(ctx, f.Model, tools, msgs, f.maxIter()); err != nil && !errors.Is(err, errLoopExhausted) {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-workflow loop: %w (rolled back)", err)
	}

	if err := f.verifyWorkflows(root); err != nil {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("fix-workflow: workflows still invalid after agent run (rolled back): %w", err)
	}

	f.writeRationale(root, failing)
	log.Info("fix-workflow: all workflow files parse as valid YAML")
	return nil
}

// verifyWorkflows re-parses all *.yml files under .github/workflows/.
func (f *FixWorkflow) verifyWorkflows(root string) error {
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to verify
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var out any
		if err := yaml.Unmarshal(data, &out); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return nil
}

func (f *FixWorkflow) maxIter() int {
	if f.MaxIterations > 0 {
		return f.MaxIterations
	}
	return defaultMaxIterations
}

func (f *FixWorkflow) writeRationale(root, failing string) {
	dir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	body := fmt.Sprintf("# fix-workflow recovery\n\nApplied at %s.\n\n## Original failure\n\n```\n%s\n```\n\nThe agent repaired .github/workflows/ YAML files until they all parse correctly.\n",
		time.Now().Format(time.RFC3339), failing)
	_ = os.WriteFile(filepath.Join(dir, "fix-workflow.md"), []byte(body), 0o644)
}

const fixWorkflowSystemPrompt = `You are a senior DevOps engineer.
GitHub Actions workflow YAML files have failed validation.

Rules:
- Read the failing workflow file(s) with read_file.
- Fix only the YAML syntax or structure errors indicated in the failure output.
- Do not change the intended behaviour of the workflow — repair, don't redesign.
- Use write_file to apply fixes (paths are repository-relative, e.g. .github/workflows/ci.yml).
- When done, reply briefly without calling any more tools.`
