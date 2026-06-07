package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultStages returns the M1 deterministic pipeline, in order, all sharing one
// Commander.
func DefaultStages(cmd Commander) []Stage {
	return []Stage{
		buildStage{cmd},
		testStage{cmd},
		dockerBuildStage{cmd},
		dockerPushStage{cmd},
	}
}

type buildStage struct{ cmd Commander }

func (buildStage) Name() string { return "build" }
func (s buildStage) Run(ctx context.Context, st *State) error {
	return runCmd(ctx, s.cmd, st, "build", st.WorkDir, "go", "build", "./...")
}

type testStage struct{ cmd Commander }

func (testStage) Name() string { return "test" }
func (s testStage) Run(ctx context.Context, st *State) error {
	return runCmd(ctx, s.cmd, st, "test", st.WorkDir, "go", "test", "./...")
}

type dockerBuildStage struct{ cmd Commander }

func (dockerBuildStage) Name() string { return "docker-build" }
func (s dockerBuildStage) Run(ctx context.Context, st *State) error {
	dockerfile := st.Dockerfile
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(st.WorkDir, dockerfile)
	}
	return runCmd(ctx, s.cmd, st, "docker-build", st.WorkDir,
		"docker", "build", "-f", dockerfile, "-t", st.ImageRef, st.WorkDir)
}

type dockerPushStage struct{ cmd Commander }

func (dockerPushStage) Name() string { return "docker-push" }
func (s dockerPushStage) Run(ctx context.Context, st *State) error {
	if !st.Push {
		return nil
	}
	return runCmd(ctx, s.cmd, st, "docker-push", st.WorkDir, "docker", "push", st.ImageRef)
}

// runCmd executes one command, records its combined output into State under
// stage, and on failure wraps the error with the command and a tail of the
// output (full output stays in State for callers that need it).
func runCmd(ctx context.Context, cmd Commander, st *State, stage, dir, bin string, args ...string) error {
	out, err := cmd.Run(ctx, dir, bin, args...)
	st.Outputs[stage] = out
	if err != nil {
		return fmt.Errorf("%s %v: %w\n%s", bin, args, err, string(tail(out, 2000)))
	}
	return nil
}

// tail returns the last n bytes of b (all of b if shorter).
func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}

// WorkflowCheckStage returns an opt-in stage that validates all *.yml files
// under .github/workflows/ as valid YAML. If the directory does not exist the
// stage is a no-op. Add it to the runner's Stages slice when
// DEPLOY_CHECK_WORKFLOW=true.
func WorkflowCheckStage(dir string) Stage { return workflowCheckStage{dir} }

type workflowCheckStage struct{ dir string }

func (workflowCheckStage) Name() string { return "workflow" }

func (s workflowCheckStage) Run(ctx context.Context, st *State) error {
	wfDir := filepath.Join(s.dir, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no workflows to check
		}
		return fmt.Errorf("workflow: read dir: %w", err)
	}

	var errs []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
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
