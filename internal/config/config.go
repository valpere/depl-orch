// Package config loads the deployment-conveyor run parameters from the
// environment. Everything is env-driven so the binary needs no rebuild to change
// target, registry, or behaviour (requirements §4: Configurability).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the parameters for one pipeline run.
//
// Trust boundary (M1): the binary is operator-controlled. WorkDir and Dockerfile
// are NOT sandboxed — a caller can point them anywhere on the host the daemon can
// read. That is acceptable while a human/CI sets the env; when the agentic layer
// (M2+) starts influencing these from repo contents, add path confinement.
type Config struct {
	WorkDir    string // directory of the repository to ship (operator-trusted)
	Dockerfile string // Dockerfile path, absolute or relative to WorkDir (operator-trusted)
	ImageRef   string // full image reference, e.g. docker.io/pereval/app:tag (not a secret)
	Push       bool   // push the built image to its registry
	Recover    bool   // enable bounded agentic recovery (fix-test) on a failed stage
	MaxRetries int    // recovery attempts per stage when Recover is on

	// Classifier routing (M3). When ClassifierModelID is empty the triage step is
	// skipped and every failure goes directly to FixTest with MODEL_ID.
	ClassifierModelID   string // CLASSIFIER_MODEL_ID — cheap model for triage
	ClassifierMaxTokens int    // CLASSIFIER_MAX_TOKENS — response cap (default 256)
	ComplexModelID      string // COMPLEX_MODEL_ID — strong model for complex failures (defaults to MODEL_ID)

	// M4: opt-in workflow YAML validation stage.
	CheckWorkflow bool // DEPLOY_CHECK_WORKFLOW — validate .github/workflows/*.yml before push

	// M5: deploy stage. Target == "" skips the stage (preserves M1 behaviour).
	DeployTarget      string // DEPLOY_TARGET: "compose" | "k8s" | "" (skip)
	DeployComposeFile string // COMPOSE_FILE — path to docker-compose file (default "docker-compose.yml")
	DeployHelmRelease string // HELM_RELEASE — Helm release name (k8s target)
	DeployHelmChart   string // HELM_CHART — Helm chart path or OCI ref (k8s target)
}

// Load reads the configuration from the environment.
//
//	DEPLOY_WORKDIR          directory to operate on         (default ".")
//	DEPLOY_DOCKERFILE       Dockerfile path                 (default "Dockerfile")
//	DEPLOY_IMAGE            full image reference             (required)
//	DEPLOY_PUSH             push the image                  (default "true")
//	CLASSIFIER_MODEL_ID     cheap triage model id           (optional; skips triage when empty)
//	CLASSIFIER_MAX_TOKENS   classifier response cap         (default 256)
//	COMPLEX_MODEL_ID        strong model for complex fixes  (default MODEL_ID)
//	DEPLOY_CHECK_WORKFLOW   validate .github/workflows/*.yml before push (default false)
//	DEPLOY_TARGET           deploy target: "compose" | "k8s" | "" (skip; default "")
//	COMPOSE_FILE            docker-compose file path                (default "docker-compose.yml")
//	HELM_RELEASE            Helm release name                       (k8s target)
//	HELM_CHART              Helm chart path or OCI ref              (k8s target)
func Load() (Config, error) {
	push, err := getenvBool("DEPLOY_PUSH", true)
	if err != nil {
		return Config{}, fmt.Errorf("DEPLOY_PUSH: %w", err)
	}
	doRecover, err := getenvBool("DEPLOY_RECOVER", false)
	if err != nil {
		return Config{}, fmt.Errorf("DEPLOY_RECOVER: %w", err)
	}
	checkWorkflow, err := getenvBool("DEPLOY_CHECK_WORKFLOW", false)
	if err != nil {
		return Config{}, fmt.Errorf("DEPLOY_CHECK_WORKFLOW: %w", err)
	}
	c := Config{
		WorkDir:             getenv("DEPLOY_WORKDIR", "."),
		Dockerfile:          getenv("DEPLOY_DOCKERFILE", "Dockerfile"),
		ImageRef:            strings.TrimSpace(os.Getenv("DEPLOY_IMAGE")),
		Push:                push,
		Recover:             doRecover,
		MaxRetries:          getenvInt("DEPLOY_MAX_RETRIES", 1),
		ClassifierModelID:   os.Getenv("CLASSIFIER_MODEL_ID"),
		ClassifierMaxTokens: getenvInt("CLASSIFIER_MAX_TOKENS", 256),
		ComplexModelID:      os.Getenv("COMPLEX_MODEL_ID"),
		CheckWorkflow:       checkWorkflow,
		DeployTarget:        strings.TrimSpace(os.Getenv("DEPLOY_TARGET")),
		DeployComposeFile:   getenv("COMPOSE_FILE", "docker-compose.yml"),
		DeployHelmRelease:   os.Getenv("HELM_RELEASE"),
		DeployHelmChart:     os.Getenv("HELM_CHART"),
	}
	if c.ImageRef == "" {
		return Config{}, fmt.Errorf("DEPLOY_IMAGE is required (e.g. docker.io/pereval/app:tag)")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvBool parses a boolean env var, returning def when unset. An invalid value
// is an error rather than a silent fallback — a typo like DEPLOY_PUSH=flase must
// not quietly flip behaviour.
func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid boolean %q: %w", v, err)
	}
	return b, nil
}
