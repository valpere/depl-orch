//go:build integration

// Package e2e runs the real deterministic pipeline against examples/sample-service,
// including a docker build and a push to the configured registry. It is gated
// behind the `integration` build tag so the default `go test ./...` stays
// hermetic (no docker required).
//
//	go test -tags integration ./test/e2e/...
//
// Override the pushed image with E2E_IMAGE; set E2E_PUSH=false to skip the push.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/valpere/depl-orch/internal/pipeline"
)

func TestPipelineEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	workDir, err := filepath.Abs("../../examples/sample-service")
	if err != nil {
		t.Fatal(err)
	}

	image := os.Getenv("E2E_IMAGE")
	if image == "" {
		image = "docker.io/pereval/depl-orch-sample:m1-test"
	}
	push := true
	if v := os.Getenv("E2E_PUSH"); v != "" {
		push, _ = strconv.ParseBool(v)
	}

	st := pipeline.NewState(workDir, "Dockerfile", image, push)
	runner := &pipeline.Runner{Stages: pipeline.DefaultStages(pipeline.DefaultCommander)}
	if err := runner.Run(context.Background(), st); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
}
