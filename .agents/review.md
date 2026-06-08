# PR #16 review — 2026-06-08

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude Sonnet 4.6

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| `internal/pipeline/healthcheck.go` | 27 | 2 | warning | 1 | DEFER | Client timeout = interval is intentional — bounds per-request time |
| `internal/pipeline/healthcheck.go` | 30 | 2 | warning | 1 | DISMISS | `//nolint:noctx` documents deliberate non-propagation of ctx |
| `internal/pipeline/healthcheck.go` | 41 | 2 | suggestion | 1 | DISMISS | Duplicate of line 30 |
| `internal/pipeline/healthcheck.go` | 43 | 2 | warning | 1 | CONFIRM | Body not drained before Close — breaks HTTP keep-alive reuse |
| `internal/pipeline/healthcheck_test.go` | 92 | 3 | warning | 1 | DISMISS | Port 1 ECONNREFUSED is reliable on Linux CI |
| `internal/pipeline/healthcheck_test.go` | 66 | 3 | suggestion | 1 | DISMISS | Intentional; ctx mid-request non-propagation is documented |
| `internal/pipeline/healthcheck_test.go` | 94 | 3 | suggestion | 1 | CONFIRM | `IntervalClampedWhenZero` used URL="" (no-op), never exercised the clamp |

## Fixes applied

1. **`internal/pipeline/healthcheck.go`** — Added `io.Copy(io.Discard, resp.Body)` before `resp.Body.Close()` so TCP keep-alive connections are returned to the pool. Added `"io"` import.

2. **`internal/pipeline/healthcheck_test.go`** — Rewrote `TestHealthCheckStage_IntervalClampedWhenZero` to use a real httptest server, assert `hcs.interval >= 2s`, and call `Run()` to confirm success. Removed hollow Name()-only check.

## Gates

go build ✓ · go vet ✓ · go test ✓ (98 tests)

## Verdict

APPROVED_WITH_FIXES
