# depl-orch — agent rules (shared: Claude Code + OpenCode)

## Project
Go project: a deployment orchestrator (build → test → docker → deploy), with
agentic recovery on failing steps. See README once it exists.

## Commands
- Build: `go build ./...`
- Test:  `go test ./...`
- Format: `gofmt -w .`  (CI rejects unformatted code)
- Lint (when configured): `go vet ./...`

## Conventions
- Idiomatic Go; small, reviewable diffs over large rewrites.
- Do not weaken or delete tests to make them pass.
- Never commit secrets. `.env` is gitignored and stays that way.

## Model / tool routing (which lane for which task)
Three lanes, cheapest capable one wins. Cost notes matter — pick by budget, not habit.

1. **Interactive Claude Code (Opus, Pro subscription)** — heavy reasoning, design,
   multi-file refactors, anything needing the full conversation. The primary driver.
2. **Headless Claude (`claude -p --model haiku|sonnet`)** — still on the Pro
   subscription (5h/weekly quota, no per-token cost), same Claude family, sees this
   `AGENTS.md`. Use for quick mechanical subtasks: boilerplate generation, small
   refactors, formatting/renames, one-shot questions. Haiku for trivial, Sonnet for
   moderate. Reserve Opus for genuinely hard work.
   - Scripting flags: `--output-format json`, `--permission-mode acceptEdits`,
     `--append-system-prompt`, `--fallback-model sonnet`.
3. **OpenCode conveyor (Gemini + Ollama Pro cloud, $20 flat)** — a *different* model
   family for independent review/tests, or to offload bulk work **without burning
   subscription quota**. Invoked via `opencode`; see handoff below. Ollama Pro caps
   at **3 concurrent cloud models** (coder/tester/documenter sit exactly at it).

Rule of thumb: trivial Claude-family work → lane 2; independent second opinion or
quota-preserving bulk → lane 3; hard/contextual → lane 1.

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
