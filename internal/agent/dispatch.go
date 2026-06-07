package agent

import (
	"context"
	"log/slog"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// StageRecovery routes recovery calls to the fixer registered for each stage.
// If no fixer is registered for the failed stage, it returns stageErr unchanged
// (fail-through), preserving deterministic behaviour for unhandled stages.
type StageRecovery struct {
	Fixers map[string]pipeline.Recoverable
	Log    *slog.Logger
}

func (s *StageRecovery) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	fixer, ok := s.Fixers[stage]
	if !ok {
		return stageErr
	}
	return fixer.Recover(ctx, st, stage, stageErr)
}
