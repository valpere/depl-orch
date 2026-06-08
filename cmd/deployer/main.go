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

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"

	"github.com/valpere/depl-orch/internal/agent"
	"github.com/valpere/depl-orch/internal/config"
	"github.com/valpere/depl-orch/internal/model"
	"github.com/valpere/depl-orch/internal/obs"
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
		if cfg.HealthURL != "" {
			// M8: wrap health-check with auto-rollback so a deployment that
			// passes docker-push but fails health is automatically reverted.
			stages = append(stages, pipeline.WithRollback(
				pipeline.HealthCheckStage(cfg.HealthURL, cfg.HealthTimeout, cfg.HealthInterval),
				deployer,
			))
		}
	} else if cfg.HealthURL != "" {
		// No deploy target — health-check runs standalone (no rollback possible).
		stages = append(stages, pipeline.HealthCheckStage(cfg.HealthURL, cfg.HealthTimeout, cfg.HealthInterval))
	}

	// M6: observability — wire metrics into the runner and register Eino callback.
	m := obs.NewMetrics()
	m.SetRunInfo(cfg.ImageRef, cfg.DeployTarget)

	st := pipeline.NewState(cfg.WorkDir, cfg.Dockerfile, cfg.ImageRef, cfg.Push)
	runner := &pipeline.Runner{
		Stages:   stages,
		Log:      log,
		Observer: &obs.PipelineObserver{Metrics: m},
	}

	// Opt-in bounded agentic recovery. Off by default → deterministic (M1 behaviour).
	if cfg.Recover {
		// Register Eino callback to record token usage. AppendGlobalHandlers is
		// not thread-safe — call once here before any model invocations.
		callbacks.AppendGlobalHandlers(obs.NewCallbackHandler(m,
			cfg.MetricsInputCostPer1M, cfg.MetricsOutputCostPer1M))
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
			mdl, err := model.New(ctx, mcfg)
			if err != nil {
				log.Error("model init", "err", err)
				os.Exit(2)
			}
			runner.Recoverer = buildStageRecovery(mdl)
			log.Info("agentic recovery enabled", "mode", "direct",
				"backend", mcfg.Backend, "model", mcfg.Model, "maxRetries", cfg.MaxRetries)
		}
	}

	runErr := runner.Run(ctx, st)

	// Push metrics regardless of pipeline outcome so partial runs are visible.
	if pushErr := obs.Push(m, cfg.MetricsPushgatewayURL, cfg.MetricsJobLabel); pushErr != nil {
		log.Warn("metrics push failed", "err", pushErr)
	}

	if runErr != nil {
		log.Error("pipeline failed", "err", runErr)
		os.Exit(1)
	}
	log.Info("pipeline ok", "image", cfg.ImageRef)
}
