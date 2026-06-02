# Claude Code Self-Configuration Brief — depl-orch (Scenario 1)

You are Claude Code, running on the user's Claude Pro subscription. Configure
THIS project so that Claude Code and OpenCode coexist in one repository, each
doing different work, then verify the setup.

**Project dir:** `~/wrk/projects/depl-orch/depl-orch`
**Repo:** https://github.com/valpere/depl-orch
**Language:** Go (TypeScript possible later)
**Starting fresh:** the user is cleaning the project; assume a near-empty repo.

## Division of labor (Scenario 1)

- **Claude Code (this tool, on Pro subscription):** interactive planner + coder.
  Heavy reasoning, design, multi-file refactors, implementation. Uses the
  subscription — no extra per-token cost. This is the primary driver.
- **OpenCode (separate tool, configured earlier):** an independent verification
  conveyor — reviewer (Gemini), tester + documenter (Ollama Pro cloud). Invoked
  when the user wants a second opinion from *different* model families, or to
  offload cheap/bulk work off the subscription.

The two tools share the git working tree and the `.agents/*.md` artifacts. They
do NOT share chat context — coordination is through files on disk.

## Critical coexistence fact

OpenCode uses the FIRST matching rules file: if both `AGENTS.md` and `CLAUDE.md`
exist, OpenCode reads ONLY `AGENTS.md` and ignores `CLAUDE.md`. Claude Code
reads only `CLAUDE.md`. To avoid instruction drift between the two tools:

- Put the **shared project rules** (build/test commands, conventions, the
  handoff protocol) in `AGENTS.md` — the tool-agnostic file both ecosystems
  understand.
- Keep `CLAUDE.md` SHORT: a Claude-Code-specific header that points to
  `AGENTS.md` as the source of truth, plus anything Claude-Code-only (subagent
  notes, hooks). This way the shared rules live in one place and can't diverge.

## Tasks

### 1. Inspect
```bash
cd ~/wrk/projects/depl-orch/depl-orch
pwd && git status && ls -la
go version 2>/dev/null
git remote -v
```
Report state. If the repo isn't initialized or the remote is missing, ask the
user before initializing.

### 2. Write `AGENTS.md` (shared source of truth, commit to git)
Run `/init` only if it won't clobber an existing curated file. Then ensure
`AGENTS.md` contains:

```markdown
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
```

Create the `.agents/` directory. Decide with the user whether `.agents/` is
committed or gitignored (commit if you want review history in git; ignore if
it's noisy scratch space).

### 3. Write `CLAUDE.md` (short, Claude-Code-specific, commit to git)
Keep it under ~40 lines so it doesn't burn context:

```markdown
# CLAUDE.md

**Source of truth for project rules is `AGENTS.md`.** Read it first; this file
only adds Claude-Code-specific notes.

## Role in this repo
Claude Code is the interactive planner + coder (running on Pro subscription).
Heavy reasoning and implementation happen here. Independent review and cheap
bulk work are delegated to OpenCode (Gemini + Ollama) — see AGENTS.md handoff.

## Subagents (optional, Claude-Code-side)
If you spin up subagents, keep them to planning/exploration. Implementation
stays in the main session unless the user asks otherwise.
```

### 4. Verify `.gitignore`
Confirm `.env` is gitignored (the OpenRouter key lives there for OpenCode).
Add if missing:
```
.env
.env.*
```
Do NOT touch any secret values.

### 5. Sanity checks — report back
- `git status` clean except intended new files.
- Confirm both files exist and that `AGENTS.md` holds the shared rules,
  `CLAUDE.md` is the short pointer.
- Confirm `go build ./...` and `go test ./...` run (even if "no Go files yet").

## Stop points
- Before `git init` or adding a remote, if the repo isn't set up — ask.
- Before the first commit — show the user the diff and the list of files to be
  committed, and confirm whether `.agents/` is committed or ignored.

When done, summarize: which files you created, what's in each, and how the user
invokes the OpenCode verification conveyor from this same repo.
