package config

import (
	"testing"
)

// setenv sets env vars for the duration of the test and restores them after.
func setenv(t *testing.T, pairs ...string) {
	t.Helper()
	for i := 0; i < len(pairs); i += 2 {
		t.Setenv(pairs[i], pairs[i+1])
	}
}

func TestLoad_RequiresDeployImage(t *testing.T) {
	t.Setenv("DEPLOY_IMAGE", "") // ensure host env doesn't satisfy the requirement
	if _, err := Load(); err == nil {
		t.Error("expected error when DEPLOY_IMAGE is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Clear all optional vars so host environment doesn't override defaults.
	setenv(t,
		"DEPLOY_IMAGE", "docker.io/pereval/app:test",
		"DEPLOY_WORKDIR", "",
		"DEPLOY_DOCKERFILE", "",
		"DEPLOY_PUSH", "",
		"DEPLOY_RECOVER", "",
		"DEPLOY_MAX_RETRIES", "",
		"CLASSIFIER_MODEL_ID", "",
		"CLASSIFIER_MAX_TOKENS", "",
		"COMPLEX_MODEL_ID", "",
		"DEPLOY_CHECK_WORKFLOW", "",
		"DEPLOY_TARGET", "",
		"COMPOSE_FILE", "",
		"HELM_RELEASE", "",
		"HELM_CHART", "",
		"METRICS_PUSHGATEWAY_URL", "",
		"METRICS_JOB_LABEL", "",
		"METRICS_INPUT_COST_PER1M", "",
		"METRICS_OUTPUT_COST_PER1M", "",
	)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkDir != "." {
		t.Errorf("WorkDir default = %q, want \".\"", cfg.WorkDir)
	}
	if cfg.Dockerfile != "Dockerfile" {
		t.Errorf("Dockerfile default = %q, want \"Dockerfile\"", cfg.Dockerfile)
	}
	if !cfg.Push {
		t.Error("Push should default to true")
	}
	if cfg.Recover {
		t.Error("Recover should default to false")
	}
	if cfg.MaxRetries != 1 {
		t.Errorf("MaxRetries default = %d, want 1", cfg.MaxRetries)
	}
	if cfg.ClassifierMaxTokens != 256 {
		t.Errorf("ClassifierMaxTokens default = %d, want 256", cfg.ClassifierMaxTokens)
	}
	if cfg.DeployComposeFile != "docker-compose.yml" {
		t.Errorf("DeployComposeFile default = %q, want \"docker-compose.yml\"", cfg.DeployComposeFile)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	setenv(t,
		"DEPLOY_IMAGE", "ghcr.io/org/app:abc123",
		"DEPLOY_WORKDIR", "/repo",
		"DEPLOY_DOCKERFILE", "build/Dockerfile",
		"DEPLOY_PUSH", "false",
		"DEPLOY_RECOVER", "true",
		"DEPLOY_MAX_RETRIES", "3",
		"CLASSIFIER_MODEL_ID", "llama3:8b",
		"CLASSIFIER_MAX_TOKENS", "512",
		"COMPLEX_MODEL_ID", "llama3:70b",
		"DEPLOY_CHECK_WORKFLOW", "true",
		"DEPLOY_TARGET", "k8s",
		"HELM_RELEASE", "myapp",
		"HELM_CHART", "./chart",
	)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ImageRef != "ghcr.io/org/app:abc123" {
		t.Errorf("ImageRef = %q", cfg.ImageRef)
	}
	if cfg.WorkDir != "/repo" {
		t.Errorf("WorkDir = %q", cfg.WorkDir)
	}
	if cfg.Push {
		t.Error("Push should be false")
	}
	if !cfg.Recover {
		t.Error("Recover should be true")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.ClassifierModelID != "llama3:8b" {
		t.Errorf("ClassifierModelID = %q", cfg.ClassifierModelID)
	}
	if cfg.ClassifierMaxTokens != 512 {
		t.Errorf("ClassifierMaxTokens = %d, want 512", cfg.ClassifierMaxTokens)
	}
	if cfg.ComplexModelID != "llama3:70b" {
		t.Errorf("ComplexModelID = %q", cfg.ComplexModelID)
	}
	if !cfg.CheckWorkflow {
		t.Error("CheckWorkflow should be true")
	}
	if cfg.DeployTarget != "k8s" {
		t.Errorf("DeployTarget = %q", cfg.DeployTarget)
	}
	if cfg.DeployHelmRelease != "myapp" {
		t.Errorf("HelmRelease = %q", cfg.DeployHelmRelease)
	}
	if cfg.DeployHelmChart != "./chart" {
		t.Errorf("HelmChart = %q", cfg.DeployHelmChart)
	}
}

func TestLoad_InvalidBool(t *testing.T) {
	setenv(t, "DEPLOY_IMAGE", "img:tag", "DEPLOY_PUSH", "flase")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid bool DEPLOY_PUSH=flase")
	}
}

func TestLoad_InvalidRecoverBool(t *testing.T) {
	setenv(t, "DEPLOY_IMAGE", "img:tag", "DEPLOY_RECOVER", "yes") // not a valid strconv.ParseBool value
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid bool DEPLOY_RECOVER=yes")
	}
}

func TestLoad_ImageRefTrimmed(t *testing.T) {
	setenv(t, "DEPLOY_IMAGE", "  img:tag  ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ImageRef != "img:tag" {
		t.Errorf("ImageRef not trimmed: %q", cfg.ImageRef)
	}
}

func TestLoad_DeployTargetTrimmed(t *testing.T) {
	setenv(t, "DEPLOY_IMAGE", "img:tag", "DEPLOY_TARGET", "  compose  ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DeployTarget != "compose" {
		t.Errorf("DeployTarget not trimmed: %q", cfg.DeployTarget)
	}
}

func TestLoad_MetricsDefaults(t *testing.T) {
	setenv(t,
		"DEPLOY_IMAGE", "img:tag",
		"METRICS_PUSHGATEWAY_URL", "",
		"METRICS_JOB_LABEL", "",
		"METRICS_INPUT_COST_PER1M", "",
		"METRICS_OUTPUT_COST_PER1M", "",
	)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MetricsPushgatewayURL != "" {
		t.Errorf("MetricsPushgatewayURL default = %q, want \"\"", cfg.MetricsPushgatewayURL)
	}
	if cfg.MetricsJobLabel != "depl-orch" {
		t.Errorf("MetricsJobLabel default = %q, want \"depl-orch\"", cfg.MetricsJobLabel)
	}
	if cfg.MetricsInputCostPer1M != 0 {
		t.Errorf("MetricsInputCostPer1M default = %v, want 0", cfg.MetricsInputCostPer1M)
	}
	if cfg.MetricsOutputCostPer1M != 0 {
		t.Errorf("MetricsOutputCostPer1M default = %v, want 0", cfg.MetricsOutputCostPer1M)
	}
}

func TestLoad_MetricsFullConfig(t *testing.T) {
	setenv(t,
		"DEPLOY_IMAGE", "img:tag",
		"METRICS_PUSHGATEWAY_URL", "http://pushgateway:9091",
		"METRICS_JOB_LABEL", "my-job",
		"METRICS_INPUT_COST_PER1M", "1.5",
		"METRICS_OUTPUT_COST_PER1M", "3.0",
	)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MetricsPushgatewayURL != "http://pushgateway:9091" {
		t.Errorf("MetricsPushgatewayURL = %q", cfg.MetricsPushgatewayURL)
	}
	if cfg.MetricsJobLabel != "my-job" {
		t.Errorf("MetricsJobLabel = %q", cfg.MetricsJobLabel)
	}
	if cfg.MetricsInputCostPer1M != 1.5 {
		t.Errorf("MetricsInputCostPer1M = %v, want 1.5", cfg.MetricsInputCostPer1M)
	}
	if cfg.MetricsOutputCostPer1M != 3.0 {
		t.Errorf("MetricsOutputCostPer1M = %v, want 3.0", cfg.MetricsOutputCostPer1M)
	}
}

func TestLoad_MetricsInvalidCostSilentlyFallsBack(t *testing.T) {
	// getenvFloat silently falls back on invalid input (consistent with getenvInt).
	setenv(t, "DEPLOY_IMAGE", "img:tag", "METRICS_INPUT_COST_PER1M", "not-a-float")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("invalid cost float should not cause error, got: %v", err)
	}
	if cfg.MetricsInputCostPer1M != 0 {
		t.Errorf("invalid cost should fall back to 0, got %v", cfg.MetricsInputCostPer1M)
	}
}
