// Command deployer runs the depl-orch deterministic pipeline (build → test →
// docker-build → docker-push) for one repository, configured from the
// environment. Exit code reflects the pipeline result (requirements §3.4).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/valpere/depl-orch/internal/config"
	"github.com/valpere/depl-orch/internal/pipeline"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// Propagate cancellation (Ctrl-C locally, SIGTERM from a cancelled CI run) to
	// the in-flight stage so docker/go don't keep running orphaned.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(2)
	}

	st := pipeline.NewState(cfg.WorkDir, cfg.Dockerfile, cfg.ImageRef, cfg.Push)
	runner := &pipeline.Runner{
		Stages: pipeline.DefaultStages(pipeline.DefaultCommander),
		Log:    log,
	}

	if err := runner.Run(ctx, st); err != nil {
		log.Error("pipeline failed", "err", err)
		os.Exit(1)
	}
	log.Info("pipeline ok", "image", cfg.ImageRef)
}
