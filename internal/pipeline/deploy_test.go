package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeDeployer records Deploy/Rollback calls and optionally fails.
type fakeDeployer struct {
	name        string
	deployErr   error
	rollbackErr error
	deployed    bool
	rolledBack  bool
}

func (f *fakeDeployer) Name() string { return f.name }
func (f *fakeDeployer) Deploy(_ context.Context, _ *State) error {
	f.deployed = true
	return f.deployErr
}
func (f *fakeDeployer) Rollback(_ context.Context, _ *State) error {
	f.rolledBack = true
	return f.rollbackErr
}

// --- ComposeDeployer ---

func TestComposeDeployer_Deploy(t *testing.T) {
	fc := &fakeCommander{}
	d := &ComposeDeployer{ComposeFile: "docker-compose.yml", Cmd: fc}
	st := newState()
	if err := d.Deploy(context.Background(), st); err != nil {
		t.Fatalf("Deploy returned error: %v", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected 1 call, got %v", fc.calls)
	}
	want := "docker compose -f docker-compose.yml up -d --force-recreate --no-build"
	if fc.calls[0] != want {
		t.Errorf("got %q, want %q", fc.calls[0], want)
	}
}

func TestComposeDeployer_Rollback(t *testing.T) {
	fc := &fakeCommander{}
	d := &ComposeDeployer{ComposeFile: "docker-compose.yml", Cmd: fc}
	st := newState()
	if err := d.Rollback(context.Background(), st); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	want := "docker compose -f docker-compose.yml down"
	if len(fc.calls) != 1 || fc.calls[0] != want {
		t.Errorf("got %v, want [%q]", fc.calls, want)
	}
}

// --- K8sDeployer ---

func TestK8sDeployer_Deploy(t *testing.T) {
	fc := &fakeCommander{}
	d := &K8sDeployer{Release: "myapp", Chart: "./chart", Cmd: fc}
	st := NewState("/work", "Dockerfile", "docker.io/pereval/app:abc123", true)
	if err := d.Deploy(context.Background(), st); err != nil {
		t.Fatalf("Deploy returned error: %v", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected 1 call, got %v", fc.calls)
	}
	call := fc.calls[0]
	for _, want := range []string{
		"helm upgrade --install myapp ./chart",
		"--set image.repository=docker.io/pereval/app",
		"--set image.tag=abc123",
	} {
		if !strings.Contains(call, want) {
			t.Errorf("call %q missing %q", call, want)
		}
	}
}

func TestK8sDeployer_Rollback(t *testing.T) {
	fc := &fakeCommander{}
	d := &K8sDeployer{Release: "myapp", Chart: "./chart", Cmd: fc}
	st := newState()
	if err := d.Rollback(context.Background(), st); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	want := "helm rollback myapp"
	if len(fc.calls) != 1 || fc.calls[0] != want {
		t.Errorf("got %v, want [%q]", fc.calls, want)
	}
}

func TestK8sDeployer_ExtraArgs(t *testing.T) {
	fc := &fakeCommander{}
	d := &K8sDeployer{Release: "r", Chart: "c", ExtraArgs: []string{"--timeout", "5m"}, Cmd: fc}
	st := newState()
	if err := d.Deploy(context.Background(), st); err != nil {
		t.Fatalf("Deploy returned error: %v", err)
	}
	if !strings.Contains(fc.calls[0], "--timeout 5m") {
		t.Errorf("extra args not passed: %q", fc.calls[0])
	}
}

// --- splitImageRef ---

func TestSplitImageRef_WithTag(t *testing.T) {
	repo, tag := splitImageRef("docker.io/myapp:v1.2.3")
	if repo != "docker.io/myapp" || tag != "v1.2.3" {
		t.Errorf("got repo=%q tag=%q", repo, tag)
	}
}

func TestSplitImageRef_NoTag(t *testing.T) {
	repo, tag := splitImageRef("myapp")
	if repo != "myapp" || tag != "latest" {
		t.Errorf("got repo=%q tag=%q", repo, tag)
	}
}

// --- NewDeployer factory ---

func TestNewDeployer_Compose(t *testing.T) {
	d, err := NewDeployer("compose", DeployConfig{ComposeFile: "compose.yml"}, &fakeCommander{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "compose" {
		t.Errorf("wrong name %q", d.Name())
	}
}

func TestNewDeployer_ComposeDefaultFile(t *testing.T) {
	d, err := NewDeployer("compose", DeployConfig{}, &fakeCommander{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cd := d.(*ComposeDeployer)
	if cd.ComposeFile != "docker-compose.yml" {
		t.Errorf("expected default file, got %q", cd.ComposeFile)
	}
}

func TestNewDeployer_K8s(t *testing.T) {
	d, err := NewDeployer("k8s", DeployConfig{HelmRelease: "rel", HelmChart: "chrt"}, &fakeCommander{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "k8s" {
		t.Errorf("wrong name %q", d.Name())
	}
}

func TestNewDeployer_K8sMissingFields(t *testing.T) {
	if _, err := NewDeployer("k8s", DeployConfig{HelmRelease: "r"}, &fakeCommander{}); err == nil {
		t.Error("expected error when HelmChart is missing")
	}
}

func TestNewDeployer_UnknownTarget(t *testing.T) {
	if _, err := NewDeployer("nomad", DeployConfig{}, &fakeCommander{}); err == nil {
		t.Error("expected error for unknown target")
	}
}

// --- DeployStage ---

func TestDeployStage_Name(t *testing.T) {
	s := DeployStage(&fakeDeployer{name: "fake"})
	if s.Name() != "deploy" {
		t.Errorf("Name() = %q, want \"deploy\"", s.Name())
	}
}

func TestDeployStage_SuccessfulDeploy(t *testing.T) {
	fd := &fakeDeployer{name: "fake"}
	st := newState()
	if err := DeployStage(fd).Run(context.Background(), st); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fd.deployed {
		t.Error("Deploy should have been called")
	}
	if fd.rolledBack {
		t.Error("Rollback must not be called on success")
	}
}

func TestDeployStage_RollsBackOnFailure(t *testing.T) {
	fd := &fakeDeployer{name: "fake", deployErr: errors.New("container crash")}
	st := newState()
	err := DeployStage(fd).Run(context.Background(), st)
	if err == nil {
		t.Fatal("expected error from failed deploy")
	}
	if !fd.rolledBack {
		t.Error("Rollback must be called when deploy fails")
	}
	if !strings.Contains(err.Error(), "container crash") {
		t.Errorf("original error should be preserved, got %v", err)
	}
	if st.Outputs["deploy"] == nil {
		t.Error("deploy output should be recorded in State")
	}
}
