package pipeline

import "context"

// Recoverable is an optional hook the Runner calls when a stage fails. An
// implementation may attempt a bounded, reversible fix (e.g. an agentic
// fix-test) and return nil to request the stage be re-run, or return an error if
// it cannot — or should not — recover that stage.
//
// It lives here, stdlib-only, so the deterministic core never imports the
// agent/LLM layer: the agent layer implements this interface, never the reverse
// (requirements §5 layering rule).
type Recoverable interface {
	Recover(ctx context.Context, st *State, stage string, stageErr error) error
}
