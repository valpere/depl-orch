# Changes

## M1 — deterministic pipeline (PR #3, Claude Code)

- `internal/pipeline/` — `Stage`/`State`/`Runner` + injectable `Commander`; four
  ordered stages (build, test, docker-build, docker-push) via `os/exec`, discrete
  args, no shell. Stdlib-only → layering rule holds by construction.
- `internal/config/` — env-driven config; strict bool parse; image trimmed+required.
- `cmd/deployer/` — wiring, JSON slog, `signal.NotifyContext`.
- `examples/sample-service/` — HTTP service (own module) + multi-stage Dockerfile.
- `test/e2e/` — build-tagged real e2e (build+push). Unit tests hermetic.
- Removed the Greet smoke scaffold (superseded).

Why: M1 is the reproducible spine the rest of the conveyor builds on (no LLM on
the pipeline path). Verified: gofmt/vet clean, `go test ./...` 6 passed; e2e
pushed `docker.io/pereval/depl-orch-sample:m1-test`. Reviewed independently by
the OpenCode reviewer (minimax-m3) → `.agents/review.md`.

## CI + docs (follow-up)

- `.github/workflows/ci.yml` — gofmt/vet/build/test on push to `main` + PRs, for
  the root module and `examples/sample-service`.
- README, AGENTS.md, requirements §6 roadmap updated to reflect M1 done + CI.
