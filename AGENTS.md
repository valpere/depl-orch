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

## Multi-tool handoff (state lives in files, not chat)
- Plans go to `.agents/plan.md` before code is written.
- Implementation changelog goes to `.agents/changes.md` (files + why).
- Independent review (run via OpenCode/Gemini) goes to `.agents/review.md`.
- Test results go to `.agents/test-report.md`.
- Doc summaries go to `.agents/summary.md`.
- The driver (Claude Code) reads these artifacts to decide done / iterate.
