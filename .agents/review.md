# M2 review

> **Note:** the OpenCode independent reviewer (minimax-m3, kimi-k2.6 fallback)
> timed out three times on this larger diff (Ollama-cloud latency, requirements
> §7). This is a SELF-review by the implementing agent — not independent. Run
> `/fix-review <PR#>` for independent multi-model eyes on the PR.

## Verdict: ready for PR; one documented trade-off, no blockers.

Verified mechanically: gofmt/vet clean, 20 hermetic unit tests, real-model e2e
fixes a broken test in ~6s, `govulncheck` 0 vulnerabilities.

## Invariants

- **Layering (§5):** `internal/pipeline` imports only stdlib — `pipeline.go`
  (context, fmt, log/slog, time), `recover.go` (context), `exec.go`
  (context, os/exec), `stages.go` (context, fmt, path/filepath). The new
  `Recoverable` interface lives here; `internal/agent` implements it and imports
  `pipeline` (one-way). **Holds.**
- **Reversibility (§2.3):** snapshot before edits; verify `go test` after; roll
  back on failure/exhaustion. Loop exhaustion is non-fatal → verify decides
  (fixed after the e2e caught a working fix being discarded). **Holds.**
- **Bounded (§2.4, §4):** `maxIterations` per loop + `MaxRetries` per stage; a
  per-call context bounds model latency. **Holds.**
- **Determinism (§2.1):** `Recoverer == nil` ⇒ exact M1 behaviour. **Holds.**

## Security

- Path-traversal guard (`tools.go:resolve`): cleans, rejects absolute + `..`
  escapes via `filepath.Rel`. Lexical only (does not follow symlinks) — matches
  the operator-controlled trust model; fine for M2.
- API keys read from env in the factory, never logged. `main.go` logs backend +
  model id only. **OK.**

## Documented trade-off (not a blocker)

- `gitRollback` (`git checkout <snap> -- .`) restores edited files but does not
  delete files the agent newly created — using `git clean -fd` would risk wiping
  legitimate untracked files in the target repo. fix-test edits existing source,
  so a stray new file on a failed attempt is the lesser evil. Commented in
  `git.go`; revisit in M4 when steps legitimately create files.

## Minor

- `gitRollback`'s error is swallowed at call sites (`_ = gitRollback`) — the
  original failure is still returned; a rollback failure is only invisible. Low
  priority; could log.
- fix-test verifies with its own `go test`, then the Runner re-runs the test
  stage (one extra `go test`). Intentional: the Runner stays authoritative.
