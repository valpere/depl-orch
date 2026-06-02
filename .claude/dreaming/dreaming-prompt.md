You are doing a **dreaming pass** for the **depl-orch** project only —
async, scheduled curation of this project's context. Sleep-time
consolidation: review what accumulated, identify patterns, suggest curation.
Read-only: produce a report, change nothing. Do NOT look at other projects.

## Project context

- **depl-orch** — a deterministic deployment conveyor in Go
  (build → test → docker-build → docker-push → deploy) with *bounded* agentic
  recovery. Control flow is plain Go; an LLM is invoked only inside individual
  recovery/generation steps, never in control flow.
- Source of truth: `AGENTS.md` (shared) + `docs/depl-orch-requirements.md`
  (full design + roadmap). `CLAUDE.md` is a short pointer to `AGENTS.md`.
- Interactive layer = Claude Code driver + OpenCode reviewer (`opencode.json`);
  this conveyor ships code, it does not write features.

## Targets

| Path | What to look for |
|------|------------------|
| `docs/depl-orch-requirements.md` §6 | Roadmap drift — does the milestone status match what's actually in the code/PRs? |
| `AGENTS.md` | Drift between documented commands/conventions and the code; stale lane/routing notes |
| `internal/pipeline/` | **Invariant audit:** does the package still import ONLY stdlib (no agent/LLM)? Any new import is a red flag. |
| `~/.claude/projects/-home-val-wrk-projects-depl-orch-depl-orch/memory/` | Stale memory files vs current code (e.g. model IDs, roadmap state) |
| `.agents/*.md` | Stale plan/changes/review/summary; scratch leaking out of `.agents/scratch/` |
| `opencode.json` | OpenCode model/role parity; do the cloud model IDs still resolve (`ollama list`)? mode:primary still correct for headless `--agent`? |
| `git log --since="3 months ago"` | Recurring failure patterns, oft-reverted commits |
| `gh pr list --state merged` | Recurring review themes worth promoting to AGENTS.md rules or tests |
| CI `.github/workflows/ci.yml` | Does CI still cover what the project needs (both modules, fmt/vet/build/test)? |

## What to find

### 1. The no-LLM-in-pipeline invariant (highest priority)
`internal/pipeline` must import only stdlib. Grep its imports; any non-stdlib
import (especially an `agent`/`model`/Eino package) is a layering violation —
flag with high severity. Agentic-off must remain fully deterministic.

### 2. Roadmap drift
Walk `docs/depl-orch-requirements.md` §6 milestones. For each marked done,
confirm the code/PRs back it; for the "next" milestone, note what's missing.
Flag any §8 open question that is now decidable.

### 3. Memory & doc staleness
Memory files and `.agents/*.md`: when last modified vs when the topic last
changed in code. Model IDs in `opencode.json` vs the memory's recorded IDs vs
`ollama list`. AGENTS.md commands vs reality.

### 4. OpenCode delegation health
- Do reviewer/tester/documenter models still resolve and stay within the
  Ollama Pro 3-concurrent cap?
- Is every `opencode run` wrapped in timeout + bounded retry (the §7
  Ollama-hang mitigation)? Flag any delegation that isn't.

### 5. Recurring PR / CI themes
`gh pr list --state merged --limit 20`. Patterns that recur → candidates for a
new AGENTS.md rule, a test, or a CI check.

### 6. Test coverage gaps
`find . -name '*_test.go'` vs production source. Pipeline/stage paths without
tests in active development = risk. Note whether the e2e integration test still
matches the deploy targets.

## Method

1. **Read** `AGENTS.md` and `docs/depl-orch-requirements.md` first — the contract.
2. **Audit** the invariant: `internal/pipeline` imports.
3. **Sample** recent commits and merged PRs.
4. **Read** memory + `.agents/*.md` selectively by mtime.
5. **Cross-compare** docs vs code vs memory for drift.
6. Output a concise report grouped by severity (blocker / should-fix / nit),
   each finding with a concrete path and a suggested action. Suggest only;
   change nothing.
