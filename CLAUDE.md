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

## Self-Learning Hard Rules

- **Run local gates before every commit** *(promoted 2026-06-08 — 2 tooling_error mistakes)*:
  `gofmt -l . | grep -v vendor` must produce no output; `go build ./... && go vet ./... && go test ./...`
  must pass. Never rely on CI to catch format errors — they cause fix commits that pollute history.
  Verify by running the gate script, not by inspection.

- **Branch before committing on this repo** *(promoted 2026-06-08 — wrong_assumption, branch protection)*:
  All repos under `~/wrk/projects/` have branch protection on `main`. Check `git branch --show-current`
  before `git add`; create a feature branch if on `main`. Never assume a direct push will succeed.
