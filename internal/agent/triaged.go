package agent

import (
	"context"
	"log/slog"

	"github.com/valpere/depl-orch/internal/pipeline"
)

// TriagedRecovery wraps two fixers with a cheap classifier that decides
// which tier to invoke — or whether to skip recovery entirely.
// It implements pipeline.Recoverable and leaves FixTest unchanged.
type TriagedRecovery struct {
	Classifier   *Classifier
	TrivialFixer pipeline.Recoverable // cheap model path
	ComplexFixer pipeline.Recoverable // strong model path
	Log          *slog.Logger
}

func (t *TriagedRecovery) Recover(ctx context.Context, st *pipeline.State, stage string, stageErr error) error {
	log := t.Log
	if log == nil {
		log = slog.Default()
	}

	class, _ := t.Classifier.Classify(ctx, stage, string(st.Outputs[stage]))
	log.Info("triage decision", "stage", stage, "class", class)

	switch class {
	case NotFixable:
		return stageErr
	case Trivial:
		return t.TrivialFixer.Recover(ctx, st, stage, stageErr)
	default: // Complex
		return t.ComplexFixer.Recover(ctx, st, stage, stageErr)
	}
}
