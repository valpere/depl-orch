// Package pipeline is the deterministic core of the deployment conveyor: an
// ordered sequence of stages (build → test → docker-build → docker-push) run by
// a plain-Go runner. Control flow is code, never an LLM (requirements §2.1); this
// package therefore never imports the agent layer.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// State is the mutable run state shared across stages. It lives in fields and the
// git working tree, never in a chat context (requirements §2.2).
type State struct {
	WorkDir    string            // repository to ship
	Dockerfile string            // Dockerfile path (abs, or relative to WorkDir)
	ImageRef   string            // full image reference to build/push
	Push       bool              // whether docker-push runs
	Outputs    map[string][]byte // stage name -> captured combined output
}

// NewState builds a State with an initialised Outputs map.
func NewState(workDir, dockerfile, imageRef string, push bool) *State {
	return &State{
		WorkDir:    workDir,
		Dockerfile: dockerfile,
		ImageRef:   imageRef,
		Push:       push,
		Outputs:    make(map[string][]byte),
	}
}

// Stage is one deterministic step. Run performs the work and records any captured
// output into st. Implementations must be ordinary code — no LLM.
type Stage interface {
	Name() string
	Run(ctx context.Context, st *State) error
}

// Runner executes stages in order, logging start/end/elapsed for each and
// stopping at the first unrecovered failure.
//
// Recovery is opt-in: with Recoverer == nil the Runner is the M1 deterministic
// fail-fast pipeline. With a Recoverer set, a failed stage is offered to it up to
// MaxRetries times — each successful Recover triggers a re-run of that stage.
type Runner struct {
	Stages     []Stage
	Log        *slog.Logger
	Recoverer  Recoverable // optional; nil = deterministic fail-fast
	MaxRetries int         // recovery attempts per stage (0 = no recovery)
}

// Run executes every stage in order. It returns the first stage error that could
// not be recovered (wrapped with the stage name), leaving later stages unrun.
func (r *Runner) Run(ctx context.Context, st *State) error {
	log := r.Log
	if log == nil {
		log = slog.Default()
	}
	for _, s := range r.Stages {
		if err := r.runStage(ctx, log, s, st); err != nil {
			return err
		}
	}
	return nil
}

// runStage runs one stage, offering a failure to the Recoverer (if any) up to
// MaxRetries times and re-running the stage after each successful recovery.
func (r *Runner) runStage(ctx context.Context, log *slog.Logger, s Stage, st *State) error {
	for attempt := 0; ; attempt++ {
		start := time.Now()
		log.Info("stage start", "stage", s.Name(), "attempt", attempt)
		err := s.Run(ctx, st)
		elapsed := time.Since(start)
		if err == nil {
			log.Info("stage ok", "stage", s.Name(), "attempt", attempt, "elapsed", elapsed.String())
			return nil
		}
		log.Error("stage failed", "stage", s.Name(), "attempt", attempt, "elapsed", elapsed.String(), "err", err)

		if r.Recoverer == nil || attempt >= r.MaxRetries {
			return fmt.Errorf("stage %q: %w", s.Name(), err)
		}
		log.Info("attempting recovery", "stage", s.Name(), "attempt", attempt)
		if rerr := r.Recoverer.Recover(ctx, st, s.Name(), err); rerr != nil {
			return fmt.Errorf("stage %q: unrecovered: %w", s.Name(), rerr)
		}
		log.Info("recovery applied, retrying stage", "stage", s.Name())
	}
}
