package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
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

