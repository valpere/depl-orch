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
