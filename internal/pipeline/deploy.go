package pipeline

import (
	"context"
	"fmt"
	"strings"
)

// Deployer applies a pushed image to a live environment and can roll the
// change back if a later step fails.
type Deployer interface {
	// Name returns a short label used in log output and error messages.
	Name() string
	Deploy(ctx context.Context, st *State) error
	Rollback(ctx context.Context, st *State) error
}

// DeployConfig holds deployer-specific configuration. Fields are left zero
// when the corresponding target is not selected.
type DeployConfig struct {
	ComposeFile string   // path to compose file; defaults to "docker-compose.yml"
	HelmRelease string   // Helm release name (k8s target)
	HelmChart   string   // Helm chart path or OCI ref (k8s target)
	HelmArgs    []string // extra args appended to helm upgrade
}

// ComposeDeployer deploys via docker compose. The compose file is expected to
// reference the image via ${IMAGE} or a fixed tag — the deployer does not
// inject env vars; the caller's environment is inherited by docker compose.
type ComposeDeployer struct {
	ComposeFile string
	Cmd         Commander
}

func (d *ComposeDeployer) Name() string { return "compose" }

func (d *ComposeDeployer) Deploy(ctx context.Context, st *State) error {
	_, err := d.Cmd.Run(ctx, st.WorkDir,
		"docker", "compose", "-f", d.ComposeFile, "up", "-d", "--force-recreate", "--no-build")
	return err
}

func (d *ComposeDeployer) Rollback(ctx context.Context, st *State) error {
	_, err := d.Cmd.Run(ctx, st.WorkDir, "docker", "compose", "-f", d.ComposeFile, "down")
	return err
}

// K8sDeployer deploys via helm upgrade --install. It parses st.ImageRef into
// image.repository and image.tag for the standard Helm chart values convention.
type K8sDeployer struct {
	Release   string
	Chart     string
	ExtraArgs []string
	Cmd       Commander
}

func (d *K8sDeployer) Name() string { return "k8s" }

func (d *K8sDeployer) Deploy(ctx context.Context, st *State) error {
	repo, tag := splitImageRef(st.ImageRef)
	args := append([]string{
		"upgrade", "--install", d.Release, d.Chart,
		"--set", "image.repository=" + repo,
		"--set", "image.tag=" + tag,
	}, d.ExtraArgs...)
	_, err := d.Cmd.Run(ctx, st.WorkDir, "helm", args...)
	return err
}

func (d *K8sDeployer) Rollback(ctx context.Context, st *State) error {
	_, err := d.Cmd.Run(ctx, st.WorkDir, "helm", "rollback", d.Release)
	return err
}

// NewDeployer returns the Deployer for target ("compose" or "k8s").
// An unknown target is an error rather than a silent no-op.
func NewDeployer(target string, cfg DeployConfig, cmd Commander) (Deployer, error) {
	switch target {
	case "compose":
		file := cfg.ComposeFile
		if file == "" {
			file = "docker-compose.yml"
		}
		return &ComposeDeployer{ComposeFile: file, Cmd: cmd}, nil
	case "k8s":
		if cfg.HelmRelease == "" || cfg.HelmChart == "" {
			return nil, fmt.Errorf("k8s deployer requires HELM_RELEASE and HELM_CHART")
		}
		return &K8sDeployer{
			Release:   cfg.HelmRelease,
			Chart:     cfg.HelmChart,
			ExtraArgs: cfg.HelmArgs,
			Cmd:       cmd,
		}, nil
	default:
		return nil, fmt.Errorf("unknown deploy target %q: want \"compose\" or \"k8s\"", target)
	}
}

// DeployStage wraps a Deployer as a pipeline.Stage. On deploy failure it
// attempts a best-effort rollback and records the original error in
// st.Outputs["deploy"].
func DeployStage(d Deployer) Stage { return deployStage{d} }

type deployStage struct{ d Deployer }

func (deployStage) Name() string { return "deploy" }

func (s deployStage) Run(ctx context.Context, st *State) error {
	if err := s.d.Deploy(ctx, st); err != nil {
		_ = s.d.Rollback(ctx, st)
		st.Outputs["deploy"] = []byte(err.Error())
		return fmt.Errorf("deploy (%s): %w", s.d.Name(), err)
	}
	return nil
}

// splitImageRef splits a full image reference into repository and tag.
// "docker.io/myapp:v1" → ("docker.io/myapp", "v1")
// "myapp" → ("myapp", "latest")
func splitImageRef(ref string) (repo, tag string) {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, "latest"
}
