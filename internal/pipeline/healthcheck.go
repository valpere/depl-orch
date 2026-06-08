package pipeline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HealthCheckStage returns a Stage that polls url with HTTP GET until a 2xx
// response is received or the context deadline is reached. It is intended to
// run immediately after DeployStage to confirm the service is up before
// declaring the pipeline successful.
//
// A zero or negative interval is clamped to 2 seconds. The stage is a no-op
// when url is empty (DEPLOY_HEALTH_URL not set).
func HealthCheckStage(url string, timeout, interval time.Duration) Stage {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return healthCheckStage{url: url, timeout: timeout, interval: interval}
}

type healthCheckStage struct {
	url      string
	timeout  time.Duration
	interval time.Duration
}

func (healthCheckStage) Name() string { return "health-check" }

func (s healthCheckStage) Run(ctx context.Context, _ *State) error {
	if s.url == "" {
		return nil
	}

	deadline := time.Now().Add(s.timeout)
	client := &http.Client{Timeout: s.interval}

	for {
		resp, err := client.Get(s.url) //nolint:noctx // context checked via deadline below
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("health-check cancelled: %w", ctx.Err())
		default:
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("health-check timed out after %s: %w", s.timeout, err)
			}
			return fmt.Errorf("health-check timed out after %s: last status %d", s.timeout, resp.StatusCode)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("health-check cancelled: %w", ctx.Err())
		case <-time.After(s.interval):
		}
	}
}
