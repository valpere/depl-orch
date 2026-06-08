# PR #14 review — 2026-06-08

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| internal/obs/metrics.go | 28 | 2 | warning | 1/3 | CONFIRM | DefBuckets tops out at 10 s; pipeline stages (docker-build, deploy) run minutes — fix with custom buckets |
| internal/obs/push.go | 13 | 2 | warning | 2/3 | DISMISS | Dual-layer defensive default — intentional; Push() guards direct API callers, config guards env-based callers |
| go.mod | 84 | 2 | warning | 1/3 | DEFER | go.yaml.in/yaml/v2 typosquat flag — transitive dep from prometheus, not introduced by this PR; audit separately |
| internal/config/config.go | 136 | 2 | warning | 1/3 | DISMISS | getenvFloat silent fallback consistent with getenvInt pattern; optional cost floats defaulting to 0 is safe |
| internal/obs/obs_test.go | 1 | 3 | suggestion | 1/3 | DISMISS | SetRunInfo test gap — L3/1v |
| internal/obs/obs_test.go | 116 | 3 | suggestion | 1/3 | DISMISS | Push httptest coverage — L3/1v |
| internal/obs/push.go | 6 | 3 | suggestion | 1/3 | DISMISS | Same as above |
| cmd/deployer/main.go | 53 | 4 | suggestion | 1/3 | DISMISS | Metrics always initialised by design — push no-ops when URL empty |
| cmd/deployer/main.go | 67 | 4 | suggestion | 1/3 | DISMISS | Eino callback scoped to recover=true by design — no agent, no tokens |
| internal/config/config.go | 133 | 4 | suggestion | 1/3 | DISMISS | Duplicate of finding on line 136 |
| internal/obs/metrics.go | 61 | 4 | suggestion | 1/3 | DISMISS | run_info cardinality — acceptable for Pushgateway (per-job replace semantics) |

## Fixes applied

1. **metrics.go:28** — Replaced `prometheus.DefBuckets` with pipeline-appropriate custom buckets `[0.1, 0.5, 1, 5, 15, 30, 60, 120, 300, 600]` covering sub-second commands through 10-minute deploys

## Gates

`go build ✓ · go vet ✓ · go test ✓ (77 tests, 6 packages)`

## Verdict

APPROVED_WITH_FIXES
