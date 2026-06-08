package obs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPipelineObserver_RecordsOkAttempt(t *testing.T) {
	m := NewMetrics()
	o := &PipelineObserver{Metrics: m}
	o.ObserveStageAttempt("build", 500*time.Millisecond, nil)

	if got := testutil.ToFloat64(m.stageAttempts.WithLabelValues("build", "ok")); got != 1 {
		t.Errorf("stageAttempts{build,ok} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.stageAttempts.WithLabelValues("build", "failed")); got != 0 {
		t.Errorf("stageAttempts{build,failed} = %v, want 0", got)
	}
}

func TestPipelineObserver_RecordsFailedAttempt(t *testing.T) {
	m := NewMetrics()
	o := &PipelineObserver{Metrics: m}
	o.ObserveStageAttempt("test", 200*time.Millisecond, errors.New("go test failed"))

	if got := testutil.ToFloat64(m.stageAttempts.WithLabelValues("test", "failed")); got != 1 {
		t.Errorf("stageAttempts{test,failed} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.stageAttempts.WithLabelValues("test", "ok")); got != 0 {
		t.Errorf("stageAttempts{test,ok} = %v, want 0", got)
	}
}

func TestCallbackHandler_RecordsTokens(t *testing.T) {
	m := NewMetrics()
	h := NewCallbackHandler(m, 0, 0)

	output := &einomodel.CallbackOutput{
		TokenUsage: &einomodel.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
		},
	}
	h.OnEnd(context.Background(), &callbacks.RunInfo{Name: "fix-test"}, output)

	if got := testutil.ToFloat64(m.agentTokens.WithLabelValues("fix-test", "prompt")); got != 100 {
		t.Errorf("agentTokens{fix-test,prompt} = %v, want 100", got)
	}
	if got := testutil.ToFloat64(m.agentTokens.WithLabelValues("fix-test", "completion")); got != 50 {
		t.Errorf("agentTokens{fix-test,completion} = %v, want 50", got)
	}
	// No cost counter should be incremented when prices are zero.
	if got := testutil.ToFloat64(m.agentCost.WithLabelValues("fix-test")); got != 0 {
		t.Errorf("agentCost{fix-test} = %v, want 0 (no price configured)", got)
	}
}

func TestCallbackHandler_RecordsCost(t *testing.T) {
	m := NewMetrics()
	// $1/1M prompt, $3/1M completion
	h := NewCallbackHandler(m, 1.0, 3.0)

	output := &einomodel.CallbackOutput{
		TokenUsage: &einomodel.TokenUsage{
			PromptTokens:     1_000_000, // exactly $1
			CompletionTokens: 1_000_000, // exactly $3
		},
	}
	h.OnEnd(context.Background(), &callbacks.RunInfo{Name: "fix-build"}, output)

	const wantCost = 4.0
	if got := testutil.ToFloat64(m.agentCost.WithLabelValues("fix-build")); got != wantCost {
		t.Errorf("agentCost{fix-build} = %v, want %v", got, wantCost)
	}
}

func TestCallbackHandler_UnknownStepWhenNameEmpty(t *testing.T) {
	m := NewMetrics()
	h := NewCallbackHandler(m, 0, 0)

	output := &einomodel.CallbackOutput{
		TokenUsage: &einomodel.TokenUsage{PromptTokens: 10},
	}
	// RunInfo.Name is empty — should fall back to "unknown" label.
	h.OnEnd(context.Background(), &callbacks.RunInfo{}, output)

	if got := testutil.ToFloat64(m.agentTokens.WithLabelValues("unknown", "prompt")); got != 10 {
		t.Errorf("agentTokens{unknown,prompt} = %v, want 10", got)
	}
}

func TestCallbackHandler_NilTokenUsageIsNoop(t *testing.T) {
	m := NewMetrics()
	h := NewCallbackHandler(m, 1.0, 3.0)

	// Non-model callback output — ConvCallbackOutput returns nil.
	h.OnEnd(context.Background(), &callbacks.RunInfo{Name: "other"}, "not-a-model-output")

	// Nothing should be recorded.
	if got := testutil.ToFloat64(m.agentTokens.WithLabelValues("other", "prompt")); got != 0 {
		t.Errorf("expected no tokens recorded for non-model output, got %v", got)
	}
}

func TestPush_NoopWhenEmpty(t *testing.T) {
	m := NewMetrics()
	if err := Push(m, "", ""); err != nil {
		t.Errorf("Push with empty URL should be a no-op, got err: %v", err)
	}
}

func TestPush_SendsToGateway(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	m := NewMetrics()
	if err := Push(m, srv.URL, "test-job"); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "/metrics/job/test-job") {
		t.Errorf("unexpected push path: %s", gotPath)
	}
}

func TestPush_DefaultJobLabelWhenEmpty(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	m := NewMetrics()
	if err := Push(m, srv.URL, ""); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if !strings.Contains(gotPath, "/metrics/job/depl-orch") {
		t.Errorf("expected default job label 'depl-orch' in path, got: %s", gotPath)
	}
}

func TestMetrics_SetRunInfo(t *testing.T) {
	m := NewMetrics()
	m.SetRunInfo("docker.io/app:v1", "compose")

	// Verify the gauge is set to 1 for the given labels.
	if got := testutil.ToFloat64(m.runInfo.WithLabelValues("docker.io/app:v1", "compose")); got != 1 {
		t.Errorf("runInfo{docker.io/app:v1,compose} = %v, want 1", got)
	}
}
