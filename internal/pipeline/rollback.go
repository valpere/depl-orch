package pipeline

import (
	"context"
	"fmt"
	"time"
)

// WithRollback wraps s so that a stage failure triggers d.Rollback before the
// error is returned. It is used to wrap HealthCheckStage when a deploy target
// is configured: if the service fails to become healthy after deploy, the
// deployer rolls back automatically.
//
// Rollback runs in a fresh context (2-minute timeout) so a cancelled pipeline
// context does not prevent the rollback from executing — same pattern as
// DeployStage.
//
// The stage Name() is forwarded from s, so observer metrics retain the inner
// stage label (e.g. "health-check").
func WithRollback(s Stage, d Deployer) Stage {
	return rollbackStage{inner: s, deployer: d}
}

type rollbackStage struct {
	inner    Stage
	deployer Deployer
}

func (r rollbackStage) Name() string { return r.inner.Name() }

func (r rollbackStage) Run(ctx context.Context, st *State) error {
	if err := r.inner.Run(ctx, st); err != nil {
		rctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if rbErr := r.deployer.Rollback(rctx, st); rbErr != nil {
			return fmt.Errorf("%w; rollback also failed: %w", err, rbErr)
		}
		return err
	}
	return nil
}
