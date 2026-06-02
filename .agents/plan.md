# Plan — M1: deterministic pipeline (build → test → docker-build → docker-push)

Source of truth: `docs/depl-orch-requirements.md` (§3.1, §5, §6 M1).
Scope: the reproducible spine. **No LLM on the pipeline path.** Eino/model
factory is M2 — not pulled in here.

## Reality check (corrects the doc's "skeleton exists")
The repo has only `go.mod` + the Greet smoke scaffold. No `internal/`, `cmd/`,
or deps. M1 is greenfield. The Greet scaffold (`greet.go`, `greet_test.go`) is
unrelated smoke plumbing → **removed** as the real code lands.

## Decisions
- **docker-push target:** Docker Hub `docker.io/pereval/<image>:<tag>` (account
  exists, daemon already logged in). Registry/image/tag are env-configurable;
  auth stays **out-of-band** (existing `docker login` / CI login step) — never in
  code or logs. ghcr.io is a future config swap (GITHUB_TOKEN), wired in M5.
- **No external deps in M1** — stdlib only (`os/exec`, `log/slog`, `context`,
  `flag`/env). Keeps the spine reproducible; deps arrive with Eino in M2.

## Files
```
cmd/deployer/main.go          wiring: load config → build pipeline → run → exit code
internal/config/config.go     env-driven config (WorkDir, image ref, Dockerfile, Push)
internal/pipeline/pipeline.go Stage interface, State, Runner (ordered, logs start/end/elapsed, stop-on-fail)
internal/pipeline/exec.go     Commander interface + osCommander (real) — injectable so stages are unit-testable
internal/pipeline/stages.go   Build, Test, DockerBuild, DockerPush stages (shell out via Commander)
internal/pipeline/*_test.go   unit tests with a fake Commander (order, stop-on-fail, output capture, elapsed)
examples/sample-service/      tiny Go HTTP service + test + hand-written Dockerfile (the e2e target)
test/e2e/e2e_test.go          //go:build integration — real e2e: build→test→docker-build→push to a test tag
```

## Key contracts
- `Stage`: `Name() string` + `Run(ctx, *State) error`. Deterministic; on failure
  captures combined output into `State` and returns a typed error.
- `State`: WorkDir, ImageRef (registry/name:tag), per-stage outputs, timings.
- `Runner`: runs stages in order; logs JSON start/end/elapsed; stops at first
  failure; returns the failing stage + captured output. No retry in M1 (YAGNI —
  deterministic stages don't retry; the retry/`Recoverable` budget arrives in M2
  when agentic recovery uses it). The layering rule holds now: `pipeline` never
  imports `agent`, so M2 slots recovery in without touching the core contract.
- `Commander`: `Run(ctx, dir, name string, args ...string) (combinedOutput []byte, err error)`.
  `osCommander` wraps `exec.CommandContext`. Stages depend on the interface →
  unit tests inject a fake, no docker/go needed for unit runs.

## Stages (deterministic, idempotent where possible)
- **build** — `go build ./...` in WorkDir.
- **test** — `go test ./...`; capture output on failure into State.
- **docker-build** — `docker build -f <Dockerfile> -t <imageRef> <ctx>`.
- **docker-push** — `docker push <imageRef>` (skipped if `Push=false`).
- Each logs start/end/elapsed (slog JSON) and wraps errors with stage name + tail
  of output.

## Test strategy
- **Unit (always run, no docker):** fake Commander asserts stage order,
  stop-on-failure, output capture, elapsed logged, and `Push=false` skips push.
- **Integration (`-tags integration`, needs docker):** real pipeline against
  `examples/sample-service` → builds image, pushes `docker.io/pereval/depl-orch-sample:m1-<shortsha>`.
  Kept out of the default `go test ./...` so CI/unit stays hermetic.

## Definition of done (per doc §9)
- Sample service runs through all four deterministic stages to a pushed image.
- No agentic/LLM code on the pipeline path; `go vet` + `gofmt` clean.
- Unit tests green by default; integration e2e verified once locally.
- Independent review by OpenCode reviewer (`minimax-m3`) → `.agents/review.md`
  before proposing the commit. Then branch + PR (no direct push to main).

## Out of scope (later milestones)
Model factory, fix-test/fix-build/dockerfile-gen, classifier (M2–M4); deploy
stage + Deployer(compose|k8s) + GitHub Actions + branch-gating (M5);
Prometheus/Grafana/Eino callbacks (M6).
