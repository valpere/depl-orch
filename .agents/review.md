# PR #11 review — 2026-06-07

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| internal/pipeline/stages.go | 11 | 1 | error | 1/3 | CONFIRM | `gopkg.in/yaml.v3` import violates `internal/pipeline` stdlib-only invariant |
| internal/agent/dispatch.go | 15 | 2 | warning | 1/3 | DISMISS | Log field unused in Recover — optional field by convention |
| internal/agent/dockerfile.go | 78 | 2 | warning | 1/3 | DISMISS | FROM check intentionally lightweight; documented trade-off |
| internal/pipeline/stages.go | 81 | 3 | warning | 1/3 | DEFER | Test gap — moot after L1 fix moved stage to agent package |
| internal/pipeline/stages.go | 85 | 3 | warning | 1/3 | DEFER | Godoc gap — moot after move |
| cmd/deployer/main.go | 58 | 4 | suggestion | 1/3 | DISMISS | Stage name constants — YAGNI |
| internal/agent/dispatch.go | 16 | 4 | suggestion | 1/3 | DISMISS | Same as L2 finding #2 |
| internal/agent/fixbuild_test.go | 69 | 4 | suggestion | 1/3 | CONFIRM | Dead code: `m` built then discarded; `m2` was used instead |
| internal/agent/fixworkflow.go | 81 | 4 | suggestion | 1/3 | CONFIRM | Only checked `.yml`, not `.yaml` — GitHub Actions supports both |
| internal/agent/fixworkflow.go | 92 | 4 | suggestion | 1/3 | CONFIRM (fold #9) | Same extension gap |
| internal/pipeline/stages.go | 99 | 4 | suggestion | 1/3 | CONFIRM (fold #1) | Same extension gap — fixed as part of stage move |

## Fixes applied

1. **[L1 CRITICAL] Move `WorkflowCheckStage` out of `internal/pipeline`**
   - Created `internal/agent/workflow_stage.go` with full implementation (yaml.v3 import is valid here)
   - Removed `WorkflowCheckStage` / `workflowCheckStage` from `internal/pipeline/stages.go`
   - Removed `os`, `strings`, `gopkg.in/yaml.v3` imports from `internal/pipeline/stages.go`
   - Updated `cmd/deployer/main.go`: `pipeline.WorkflowCheckStage` → `agent.WorkflowCheckStage`

2. **[L4] Remove dead code in `fixbuild_test.go`**
   - Removed `m := &fakeErrorModel{err: errLoopExhausted}` and `_ = m`
   - Inlined `&fakeErrorModel{err: context.DeadlineExceeded}` directly

3. **[L4] Add `.yaml` extension support in `fixworkflow.go:verifyWorkflows`**
   - Changed single `filepath.Ext(e.Name()) != ".yml"` to `ext != ".yml" && ext != ".yaml"`

4. **[L4] Add `.yaml` extension support in new `workflow_stage.go`**
   - New stage checks both `.yml` and `.yaml` from the start

## Gates

`go build ✓ · go vet ✓ · go test ✓ (46 tests, 5 packages)`

## Verdict

APPROVED_WITH_FIXES
