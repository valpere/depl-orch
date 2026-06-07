package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// GenerateDockerfile is a bounded, reversible agentic recovery step for a
// failing docker-build stage. It generates a Dockerfile if one is missing or
// repairs a broken one. It implements pipeline.Recoverable.
//
// Verification is intentionally lightweight (file exists + contains "FROM"):
// the docker binary is not available in unit tests. The pipeline's docker-build
// stage re-run is the authoritative check.
type GenerateDockerfile struct {
	Model         model.ToolCallingChatModel
	MaxIterations int
	Log           *slog.Logger
}

func (g *GenerateDockerfile) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	if stage != "docker-build" {
		return stageErr
	}
	log := g.Log
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

	failing := string(st.Outputs["docker-build"])
	dockerfilePath := st.Dockerfile
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(root, dockerfilePath)
	}

	msgs := []*schema.Message{
		schema.SystemMessage(generateDockerfileSystemPrompt),
		schema.UserMessage(fmt.Sprintf(
			"Expected Dockerfile path: %s\n\ndocker build failed:\n\n%s\n\nGenerate or fix the Dockerfile so docker build succeeds.",
			st.Dockerfile, failing,
		)),
	}

	log.Info("generate-dockerfile: starting recovery loop", "maxIterations", g.maxIter())
	if _, err := runToolLoop(ctx, g.Model, tools, msgs, g.maxIter()); err != nil && !errors.Is(err, errLoopExhausted) {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("generate-dockerfile loop: %w (rolled back)", err)
	}

	// Lightweight verify: Dockerfile must exist and contain a FROM instruction.
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("generate-dockerfile: Dockerfile not found at %s after agent run (rolled back): %w", dockerfilePath, err)
	}
	if !strings.Contains(string(content), "FROM") {
		_ = gitRollback(ctx, root, snapshot)
		return fmt.Errorf("generate-dockerfile: Dockerfile at %s lacks a FROM instruction (rolled back)", dockerfilePath)
	}

	g.writeRationale(root, failing)
	log.Info("generate-dockerfile: Dockerfile present with FROM instruction")
	return nil
}

func (g *GenerateDockerfile) maxIter() int {
	if g.MaxIterations > 0 {
		return g.MaxIterations
	}
	return defaultMaxIterations
}

func (g *GenerateDockerfile) writeRationale(root, failing string) {
	dir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	body := fmt.Sprintf("# generate-dockerfile recovery\n\nApplied at %s.\n\n## Original failure\n\n```\n%s\n```\n\nThe agent generated or repaired the Dockerfile until it contained a valid FROM instruction.\n",
		time.Now().Format(time.RFC3339), failing)
	_ = os.WriteFile(filepath.Join(dir, "generate-dockerfile.md"), []byte(body), 0o644)
}

const generateDockerfileSystemPrompt = `You are a senior Go engineer and Docker expert.
A docker build stage has failed, typically because the Dockerfile is missing or broken.

Rules:
- If the Dockerfile is missing, generate a minimal, correct one for a Go service.
- If the Dockerfile is present but broken, repair it with the minimal change.
- If the failure is clearly NOT Dockerfile-related (e.g. network error, registry auth),
  take no action and reply without calling any tools.
- Standard Go Dockerfile pattern: multi-stage build (golang:1.26-alpine builder,
  scratch or alpine final), COPY go.mod/go.sum first for layer caching, CGO_ENABLED=0.
- Use write_file to write the Dockerfile (path is repository-relative).
- When done, reply briefly without calling any more tools.`
