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
// stopping at the first failure.
type Runner struct {
	Stages []Stage
	Log    *slog.Logger
}

// Run executes every stage in order. It returns the first stage error (wrapped
// with the stage name), leaving later stages unrun.
func (r *Runner) Run(ctx context.Context, st *State) error {
	log := r.Log
	if log == nil {
		log = slog.Default()
	}
	for _, s := range r.Stages {
		start := time.Now()
		log.Info("stage start", "stage", s.Name())
		err := s.Run(ctx, st)
		elapsed := time.Since(start)
		if err != nil {
			log.Error("stage failed", "stage", s.Name(), "elapsed", elapsed.String(), "err", err)
			return fmt.Errorf("stage %q: %w", s.Name(), err)
		}
		log.Info("stage ok", "stage", s.Name(), "elapsed", elapsed.String())
	}
	return nil
}
