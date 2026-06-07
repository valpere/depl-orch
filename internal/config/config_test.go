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
	if _, err := Load(); err == nil {
		t.Error("expected error when DEPLOY_IMAGE is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	setenv(t, "DEPLOY_IMAGE", "docker.io/pereval/app:test")
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
