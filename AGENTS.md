# depl-orch — agent rules (shared: Claude Code + OpenCode)

## Project
Go project: a deployment orchestrator (build → test → docker → deploy), with
agentic recovery on failing steps. See README once it exists.

## Commands
- Build: `go build ./...`
- Test:  `go test ./...`  (unit tests are hermetic — no docker needed)
- Format: `gofmt -w .`  (CI rejects unformatted code)
- Lint: `go vet ./...`
- E2E (real docker build+push): `go test -tags integration ./test/e2e/...`
- CI: `.github/workflows/ci.yml` runs gofmt/vet/build/test on push to `main` and
  every PR, for the root module and `examples/sample-service` (its own module).

## Conventions
- Idiomatic Go; small, reviewable diffs over large rewrites.
- Do not weaken or delete tests to make them pass.
- Never commit secrets. `.env` is gitignored and stays that way.

## Model / tool routing (which lane for which task)
Three lanes. Match the lane to the work; cheapest *capable* one wins.

1. **Driver — interactive Claude Code, Opus or Sonnet (Pro subscription).** The
   primary orchestrator + planner + coder. Holds the full conversation, plans
   **inline** (planning is context-heavy — never push it headless), and judges
   every delegated result. Use Opus for hard reasoning / multi-file design; drop
   to Sonnet to save quota when the work is moderate. Pick the strongest model
   you're willing to spend — the driver must be **≥** anything it orchestrates.
2. **Tool lane — headless Haiku (`claude -p --model haiku`).** Same Pro
   subscription (5h/weekly quota, no per-token), sees this `AGENTS.md`. For
   bounded, deterministic, no-judgment mechanics: formatting, renames,
   boilerplate, running a command and reporting. Haiku is stateless (cold start
   per call) — pass it everything it needs; don't hand it work that needs context
   or judgment.
   - Scripting flags: `--output-format json`, `--permission-mode acceptEdits`,
     `--append-system-prompt`, `--fallback-model sonnet`.
3. **Review lane — OpenCode (Ollama Pro cloud, $20 flat).** A **different model
   family** (non-Claude) for independent review/tests — independence is the whole
   point, so this must NOT be Claude. Also absorbs quota-preserving bulk. Invoked
   via `opencode run --agent <role>`; see handoff below. Reviewer = minimax-m3;
   Ollama Pro caps at **3 concurrent cloud models**.

Anti-patterns: don't let a weaker model orchestrate a stronger one (the
orchestrator must judge the output it receives), and don't make a strong model a
*headless* planner — headless is stateless, and planning needs the live context.

Rule of thumb: hard/contextual → lane 1; mechanical no-judgment → lane 2;
independent (cross-family) review or bulk → lane 3.

## Milestones

| # | Name | Status |
|---|------|--------|
| M1 | Deterministic pipeline (build→test→docker-build→docker-push) | ✅ done |
| M2 | Model factory + bounded fix-test recovery + git rollback | ✅ done |
| M3 | Classifier-based triage (cheap model routes easy vs complex failures) | ✅ done |
| M4 | Extended recovery: fix-build, generate-dockerfile, fix-workflow | ✅ done |
| M5 | Deploy stage (Compose + k8s/Helm) + GH Actions deploy workflow | ✅ done |
| M6 | Observability: Prometheus Pushgateway + Grafana dashboard + Eino token callbacks | ✅ done |
| M7 | Health-check gate: poll /healthz after deploy until 2xx or timeout | ✅ done |

## Multi-tool handoff (state lives in files, not chat)
- Plans go to `.agents/plan.md` before code is written.
- Implementation changelog goes to `.agents/changes.md` (files + why).
- Independent review (run via OpenCode reviewer) goes to `.agents/review.md`.
- Test results go to `.agents/test-report.md`.
- Doc summaries go to `.agents/summary.md`.
- The driver (Claude Code) reads these artifacts to decide done / iterate.
- These canonical files are committed as handoff history. Transient scratch —
  raw diffs, A/B variants, run logs — goes to `.agents/scratch/` or an
  `_`-prefixed name (both gitignored). Never commit scratch.
