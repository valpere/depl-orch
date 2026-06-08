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

	"github.com/valpere/depl-orch/internal/obs"
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

	m := obs.NewMetrics()
	st := pipeline.NewState(workDir, "Dockerfile", image, push)
	runner := &pipeline.Runner{
		Stages:   pipeline.DefaultStages(pipeline.DefaultCommander),
		Observer: &obs.PipelineObserver{Metrics: m},
	}
	if err := runner.Run(context.Background(), st); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Regression: observer must not break the pipeline and must record each stage.
	// We expect at least 3 stages (build, test, docker-build); push adds a 4th.
	minStages := 3
	if push {
		minStages = 4
	}
	mfs, err := m.Gather()
	if err != nil {
		t.Fatalf("metrics gather: %v", err)
	}
	var attemptTotal float64
	for _, mf := range mfs {
		if mf.GetName() == "depl_orch_stage_attempts_total" {
			for _, metric := range mf.GetMetric() {
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == "status" && lp.GetValue() == "ok" {
						attemptTotal += metric.GetCounter().GetValue()
					}
				}
			}
		}
	}
	if int(attemptTotal) < minStages {
		t.Errorf("observer recorded %v ok attempts, want at least %d", attemptTotal, minStages)
	}
}
