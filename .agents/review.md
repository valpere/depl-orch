# PR #20 review — 2026-07-03

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · gemma4:31b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude Sonnet 4.6

Round 2 (kimi-k2.6:cloud) returned prose analysis instead of the required JSON
array — parsed as 0 findings per protocol. It raised a minor observation about
`extractJSON` not handling top-level JSON arrays, but the only current caller
(`Classifier.Classify`) always parses a JSON object, so no fix was needed.

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| `internal/framework/lifecycle/lifecycle.go` | 2, 79 | 1 | error | 1 | CONFIRM | Package + `Register` docstrings claimed services start "in registration order" (implying sequential); `App.Run` actually launches them concurrently via goroutines. Fixed docstrings to say "start concurrently (launched in registration order)". |
| `internal/agent/history.go` | 47 | 2 | error | 1 | CONFIRM | `bufio.NewScanner` default 64KB max token size silently drops `fix-history.jsonl` entries whose `Diff` (a full `git diff`) exceeds it — exactly the non-trivial fixes the few-shot feature most needs. Raised buffer to 10MB via `scanner.Buffer`. |
| `internal/web/server.go` | 91 | 2 | error | 1 | CONFIRM | Same 64KB scanner limit in the dashboard's history render — same fix. |
| `Makefile` | 50 | 1 | warning | 1 | CONFIRM | `clean` target's `find . -name "_*"` scans the whole repo tree, not just scratch. Checked `.gitignore`: it only covers `.agents/_*`, not a repo-wide pattern — the broad find could delete unrelated `_`-prefixed user directories/files elsewhere in the tree. Scoped both `find` invocations to `.agents/`. |
| `internal/framework/lifecycle/lifecycle.go` | 111 | 2 | error | 1 | DISMISS | Claimed `wg.Go` "is not valid Go syntax" (stale training data — `sync.WaitGroup.Go` shipped in Go 1.25). `go.mod` pins `go 1.26.4`; this exact code already built/vet/tested clean before this review ran. |
| `internal/framework/lifecycle/lifecycle.go` | 104 | 2 | warning | 1 | DISMISS | Same false premise as above (asks to verify Go ≥1.25 — already satisfied at 1.26.4). |
| `internal/agent/history.go` | 23, 32 | 2 | warning | 1 | CONFIRM | `appendHistory` silently swallowed `MkdirAll`/`OpenFile`/`Write` errors — a failed persist left no trace. Added a `*slog.Logger` parameter (already in scope at both call sites in `fixbuild.go`/`fixtest.go`) and logs each failure as a warning. |
| `internal/web/server.go` | 46 | 2 | warning | 1 | CONFIRM | Real race: the goroutine calling `ListenAndServe` isn't guaranteed to have bound the port before `Start` returns via `ctx.Done()`; `Stop`'s `Shutdown` would then no-op against an unbound server, leaking the later-bound listener. Fixed by binding via `net.Listen` synchronously in `Start`, then `Serve(ln)` in the goroutine. |
| `internal/web/server.go` | 103, 80 | 2 | warning | 1 | DEFER | Full-file reparse of `fix-history.jsonl` on every HTTP request. Real but premature optimization for a low-traffic internal dev tool at current scale; revisit if history grows large or the dashboard sees real traffic. |
| `cmd/deployer/main.go` | 32, 34 | 2 | warning | 1 | DEFER | `pipelineSvc.Start` reaches into the caller's `cancel` to trigger app shutdown on success — real coupling concern, functions correctly today. A `OneShot` service abstraction would be cleaner but is a bigger design change than this pass should make. |
| `internal/agent/history.go`, `util.go`, `internal/framework/lifecycle/lifecycle.go`, `internal/web/server.go` | 1 | 3 | suggestion | 1 each | DEFER | Zero test coverage across all 4 new files/packages — already flagged in this PR's own description as a known follow-up. `lifecycle.App.Run`'s concurrent start/stop/error-propagation behavior and the listener-bind race just fixed both warrant dedicated regression tests; too large to fold into this pass. |
| `internal/web/server.go` | 95 | 4 | suggestion | 1 | CONFIRM | O(n²) history build via repeated slice prepend. Changed to append + single reverse pass. |

## Fixes applied

1. **`internal/framework/lifecycle/lifecycle.go`** — corrected package + `Register` docstrings to describe actual concurrent-start behavior.
2. **`internal/agent/history.go`, `internal/web/server.go`** — `scanner.Buffer(...)` raises the max line size to 10MB so large diffs aren't silently dropped.
3. **`Makefile`** — scoped `clean` target's `find` invocations to `.agents/` (matches what `.gitignore` actually covers).
4. **`internal/agent/history.go`, `fixbuild.go`, `fixtest.go`** — `appendHistory` now takes a `*slog.Logger` and logs persistence failures instead of swallowing them.
5. **`internal/web/server.go`** — `Start` binds the listener synchronously via `net.Listen` before the `Serve` goroutine starts, closing the bind-race window; history building changed from O(n²) prepend to O(n) append+reverse.

## Gates

go build ✓ · go vet ✓ · go test ✓ (107 tests)

## Verdict

APPROVED_WITH_FIXES
