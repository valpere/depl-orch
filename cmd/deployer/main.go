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

	einomodel "github.com/cloudwego/eino/components/model"

	"github.com/valpere/depl-orch/internal/agent"
	"github.com/valpere/depl-orch/internal/config"
	"github.com/valpere/depl-orch/internal/model"
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

	stages := pipeline.DefaultStages(pipeline.DefaultCommander)
	if cfg.CheckWorkflow {
		stages = append(stages, agent.WorkflowCheckStage(cfg.WorkDir))
	}
	if cfg.DeployTarget != "" {
		deployer, err := pipeline.NewDeployer(cfg.DeployTarget, pipeline.DeployConfig{
			ComposeFile: cfg.DeployComposeFile,
			HelmRelease: cfg.DeployHelmRelease,
			HelmChart:   cfg.DeployHelmChart,
		}, pipeline.DefaultCommander)
		if err != nil {
			log.Error("deploy target", "err", err)
			os.Exit(2)
		}
		stages = append(stages, pipeline.DeployStage(deployer))
	}

	st := pipeline.NewState(cfg.WorkDir, cfg.Dockerfile, cfg.ImageRef, cfg.Push)
	runner := &pipeline.Runner{
		Stages: stages,
		Log:    log,
	}

	// Opt-in bounded agentic recovery. Off by default → deterministic (M1 behaviour).
	if cfg.Recover {
		mcfg, err := model.Load()
		if err != nil {
			log.Error("model config", "err", err)
			os.Exit(2)
		}
		runner.MaxRetries = cfg.MaxRetries

		// M4: build a StageRecovery that dispatches to the right fixer per stage.
		buildStageRecovery := func(m einomodel.ToolCallingChatModel) pipeline.Recoverable {
			return &agent.StageRecovery{
				Fixers: map[string]pipeline.Recoverable{
					"test":         &agent.FixTest{Model: m, Log: log},
					"build":        &agent.FixBuild{Model: m, Log: log},
					"docker-build": &agent.GenerateDockerfile{Model: m, Log: log},
					"workflow":     &agent.FixWorkflow{Model: m, Log: log},
				},
				Log: log,
			}
		}

		if cfg.ClassifierModelID != "" {
			// M3: triage before recovery — cheap classifier decides which tier to use.
			classifierCfg := mcfg
			classifierCfg.Model = cfg.ClassifierModelID
			classifierCfg.MaxTokens = cfg.ClassifierMaxTokens
			cm, err := model.New(ctx, classifierCfg)
			if err != nil {
				log.Error("classifier model init", "err", err)
				os.Exit(2)
			}

			trivialModel, err := model.New(ctx, mcfg)
			if err != nil {
				log.Error("trivial fixer model init", "err", err)
				os.Exit(2)
			}

			complexCfg := mcfg
			if cfg.ComplexModelID != "" {
				complexCfg.Model = cfg.ComplexModelID
			}
			complexModel, err := model.New(ctx, complexCfg)
			if err != nil {
				log.Error("complex fixer model init", "err", err)
				os.Exit(2)
			}

			runner.Recoverer = &agent.TriagedRecovery{
				Classifier:   &agent.Classifier{Model: cm, Log: log},
				TrivialFixer: buildStageRecovery(trivialModel),
				ComplexFixer: buildStageRecovery(complexModel),
				Log:          log,
			}
			log.Info("agentic recovery enabled", "mode", "triaged",
				"classifier", cfg.ClassifierModelID, "trivial", mcfg.Model,
				"complex", complexCfg.Model, "maxRetries", cfg.MaxRetries)
		} else {
			m, err := model.New(ctx, mcfg)
			if err != nil {
				log.Error("model init", "err", err)
				os.Exit(2)
			}
			runner.Recoverer = buildStageRecovery(m)
			log.Info("agentic recovery enabled", "mode", "direct",
				"backend", mcfg.Backend, "model", mcfg.Model, "maxRetries", cfg.MaxRetries)
		}
	}

	if err := runner.Run(ctx, st); err != nil {
		log.Error("pipeline failed", "err", err)
		os.Exit(1)
	}
	log.Info("pipeline ok", "image", cfg.ImageRef)
}
