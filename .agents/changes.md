# Changes

## M2 — model factory + fix-test recovery (Claude Code)

- `internal/model/` — backend-agnostic `ToolCallingChatModel` factory via Eino
  (`ollama|openai|anthropic`), env-driven config. Pinned eino v0.9.4 + eino-ext.
- `internal/agent/` — first bounded agentic step:
  - `tools.go` — repo-scoped `read_file`/`write_file` (path-traversal guard).
  - `loop.go` — generic bounded generate→dispatch-tool-calls loop. Budget
    exhaustion is non-fatal (caller verifies).
  - `git.go` — `git stash create` snapshot + `checkout` rollback.
  - `fixtest.go` — `FixTest` implements `pipeline.Recoverable`: snapshot → bounded
    model loop (edits source, never tests) → verify `go test` → keep or **roll back**.
- `internal/pipeline/recover.go` — `Recoverable` interface (stdlib only) + Runner
  recovery wiring (`Recoverer`+`MaxRetries`); nil recoverer = M1 fail-fast.
- `cmd/deployer` + `internal/config` — opt-in recovery via `DEPLOY_RECOVER`
  (+ `MODEL_*` env). Default off → deterministic.
- Security: bumped go 1.26.4, x/net 0.55.0, x/crypto 0.52.0 → govulncheck clean.

Why: M2 adds judgment INSIDE a stage without leaking it into control flow —
`internal/pipeline` still imports only stdlib. Verified: gofmt/vet clean, 20 unit
tests (fake model + temp git repo cover fix + rollback); real-model e2e under
`-tags integration`. Independent review by OpenCode reviewer (minimax-m3).

## M1 + CI (earlier)

- M1: deterministic pipeline (build/test/docker-build/docker-push), no LLM (PR #3).
- CI: `.github/workflows/ci.yml` gofmt/vet/build/test (PR #4).
- depl-orch-scoped weekly dreaming pass (PR #5).
