# PR #12 review — 2026-06-07

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| internal/pipeline/deploy.go | 9 | 1 | warning | 1/3 | CONFIRM | Deployer comment said "if a later step fails" — rollback is called on Deploy failure, not a later stage |
| internal/pipeline/deploy.go | 109 | 2 | error | 1/3 | CONFIRM | `Rollback(ctx, st)` reused potentially-cancelled ctx; fixed with fresh `context.WithTimeout(context.Background(), 2m)` |
| internal/pipeline/deploy.go | 120 | 2 | error | 1/3 | CONFIRM | `splitImageRef` returned wrong result for `registry:5000/image` (no explicit tag): port colon treated as tag separator |
| internal/pipeline/deploy.go | 138 | 2 | error | 1/3 | CONFIRM (fold) | Same bug, folded into fix for line 120 |
| .github/workflows/deploy.yml | 53 | 2 | warning | 1/3 | CONFIRM | Comment implied docker/login-action was already present; clarified it must be added |
| internal/pipeline/deploy_test.go | 209 | 3 | warning | 1/3 | CONFIRM | Missing test for `registry:port/image` without explicit tag |
| internal/pipeline/deploy_test.go | 95 | 3 | warning | 1/3 | CONFIRM (fold) | Same gap, folded into new tests |
| cmd/deployer/main.go | 43 | 4 | suggestion | 1/3 | DISMISS | Log message adequate |
| internal/pipeline/deploy.go | 24 | 4 | suggestion | 1/3 | DISMISS | HelmArgs intentionally env-var-free (no safe single-string parse) |

## Fixes applied

1. **[L1] Deployer comment** — "if a later step fails" → "if Deploy fails"
2. **[L2] Rollback context** — replaced `Rollback(ctx, st)` with fresh `context.WithTimeout(context.Background(), 2*time.Minute)` so a timed-out deploy ctx doesn't immediately kill rollback
3. **[L2] splitImageRef bug** — added heuristic: if the substring after the last colon contains a `/`, it is a registry port, not a tag; returns `"latest"` instead
4. **[L2] Workflow comment** — clarified the docker/login-action comment to be instructional rather than implying it exists
5. **[L3] Tests** — added `TestSplitImageRef_RegistryPortWithTag` and `TestSplitImageRef_RegistryPortNoTag`

## Gates

`go build ✓ · go vet ✓ · go test ✓ (63 tests, 5 packages)`

## Verdict

APPROVED_WITH_FIXES
