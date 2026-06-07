package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// WorkflowCheckStage returns an opt-in pipeline.Stage that validates all
// *.yml and *.yaml files under .github/workflows/ as valid YAML. If the
// directory does not exist the stage is a no-op. Wire it in with
// DEPLOY_CHECK_WORKFLOW=true; pair with FixWorkflow for recovery.
func WorkflowCheckStage(dir string) pipeline.Stage { return workflowCheckStage{dir} }

type workflowCheckStage struct{ dir string }

func (workflowCheckStage) Name() string { return "workflow" }

func (s workflowCheckStage) Run(_ context.Context, st *pipeline.State) error {
	wfDir := filepath.Join(s.dir, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no workflows directory — no-op
		}
		return fmt.Errorf("workflow: read dir: %w", err)
	}

	var errs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(wfDir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		var out any
		if err := yaml.Unmarshal(data, &out); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", e.Name(), err))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	msg := "workflow YAML validation failed:\n" + strings.Join(errs, "\n")
	st.Outputs["workflow"] = []byte(msg)
	return fmt.Errorf("%s", msg)
}
