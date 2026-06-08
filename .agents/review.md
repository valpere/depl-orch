# PR #17 review — 2026-06-08

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude Sonnet 4.6

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| `internal/pipeline/rollback.go` | 36 | 2 | warning | 1 | CONFIRM | `rbErr` used `%v` — callers couldn't `errors.Is` rollback failure; fixed to `%w` |
| `internal/pipeline/rollback.go` | 20 | 2 | suggestion | 1 | DISMISS | Nil guard on internal code contradicts YAGNI; main.go always provides non-nil |
| `cmd/deployer/main.go` | 52 | 3 | suggestion | 1 | DEFER | Testing main() wiring is out of scope; WithRollback logic fully covered in rollback_test.go |
| `internal/pipeline/healthcheck_test.go` | 133 | 4 | suggestion | 1 | DISMISS | Both files are `package pipeline`; funcStage accessible, no duplication |
| `internal/pipeline/healthcheck_test.go` | 154 | 4 | suggestion | 1 | DISMISS | `slog.Default()` is fine in tests |
| `internal/pipeline/rollback.go` | 33 | 4 | suggestion | 1 | DISMISS | `2*time.Minute` matches deploy.go's identical pattern intentionally |
| `internal/pipeline/rollback.go` | 35 | 4 | suggestion | 1 | DEFER | Logging needs logger access — changes WithRollback signature; out of scope |

## Fixes applied

1. **`internal/pipeline/rollback.go:36`** — Changed `%v` → `%w` for `rbErr` so callers can use `errors.Is(err, rbErr)` to detect rollback failures. Go 1.26 supports multiple `%w` in `fmt.Errorf`.

2. **`internal/pipeline/rollback_test.go`** — Updated `TestWithRollback_BothErrorsReportedWhenRollbackFails` to assert `errors.Is(err, rbErr)` (verifies `%w` wrapping); removed now-unnecessary `strings` import.

## Gates

go build ✓ · go vet ✓ · go test ✓ (107 tests)

## Verdict

APPROVED_WITH_FIXES
