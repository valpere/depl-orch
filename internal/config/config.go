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
}

// Load reads the configuration from the environment.
//
//	DEPLOY_WORKDIR     directory to operate on   (default ".")
//	DEPLOY_DOCKERFILE  Dockerfile path           (default "Dockerfile")
//	DEPLOY_IMAGE       full image reference       (required)
//	DEPLOY_PUSH        push the image            (default "true")
func Load() (Config, error) {
	push, err := getenvBool("DEPLOY_PUSH", true)
	if err != nil {
		return Config{}, fmt.Errorf("DEPLOY_PUSH: %w", err)
	}
	c := Config{
		WorkDir:    getenv("DEPLOY_WORKDIR", "."),
		Dockerfile: getenv("DEPLOY_DOCKERFILE", "Dockerfile"),
		ImageRef:   strings.TrimSpace(os.Getenv("DEPLOY_IMAGE")),
		Push:       push,
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
