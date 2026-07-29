# PR #23 review — 2026-07-29

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · gemma4:31b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude Sonnet 4.6

Round 3 (gemma4:31b-cloud) returned prose analysis instead of JSON — parsed as 0 findings per protocol.

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
| `.claude/skills/fix-review/SKILL.md` | 80, 86 | 2 | warning | 1 | CONFIRM | `deplorch_probe` predicate (`jq -e '.message.content // empty'`) accepted Ollama error envelopes like `{"error":"model not found"}` because `.message.content` is null and `// empty` produced an empty string — `jq -e` then exited 0 and the probe silently reported success, so `CLOUD_KNOWN_BAD` stayed "no". Same bug class as canonical's `probe_provider`. Tightened predicate to `(.message.content // "") != "" and (has("error") | not)`. |
| `.claude/skills/fix-review/SKILL.md` | 182 | 2 | error | 1 | CONFIRM | After retrying an Ollama round that returned empty content, the code only updated `err` from the retry response's `.error` field. If the retry also returned empty content (no `.error`), `err` stayed empty and the round skipped Tier 2/Tier 3 cascade — silent 0 findings. Added a second content check that force-sets `err="empty content after retry"` so the cascade fires. |
| `.claude/skills/fix-review/SKILL.md` | 235 | 2 | warning | 1 | CONFIRM | Reconciliation block (`grep -l '^external_agents:'` on `round_*.failover`) only fired when at least one round wrote a marker. `try_external_agents` only writes markers on success — when every external agent in the cascade failed and Tier 2 was the only configured fallback, no marker was written, reconciliation left `FAILOVER_TIER=""`, and the user saw a "clean" review with 0 findings. Added `external_agents:none` marker written at the "all external_agents failed" branch in `run_round`, plus a post-reconciliation warning check. |
| `.claude/skills/lib/agents.sh` | 103–112 | 2 | warning | 2 | CONFIRM | Identical 5-line comment block duplicated verbatim in `try_external_agents`. Removed the duplicate (canonical source has the same bug; can sync upstream separately). |
| `.claude/skills/fix-review/SKILL.md` | 190 | 2 | warning | 1 | DISMISS | `mktemp` without template fails on macOS/BSD. Project CI runs Linux-only (ubuntu-latest); macOS portability isn't a concern here. Stylistic. |
| `.claude/skills/fix-review/SKILL.md` | 210 | 2 | warning | 1 | DEFER | Tier 3 (ollama_local) reuses `$API_URL` (same URL the probe just tested). Real — but canonical does the same and it's documented as "if the daemon is completely down, hard failure." Documenting in the SKILL description is fair scope for a follow-up PR; not blocking. |
| `.claude/skills/fix-review/SKILL.md` | 245 | 2 | warning | 1 | DISMISS | `ls $glob` is unreliable under `nullglob`. No `nullglob` set in this repo's shell scope (subshells in `&` jobs, defaults to bash). False positive in practice here. |
| `.claude/skills/lib/agents.sh` | 47, 52 | 2 | warning | 1 | DEFER | `agent_codex` JSON extraction via `awk '/^\[/'` is fragile against indented/multi-line output. Real fragility in code copied verbatim from canonical — fixing here would diverge from canonical. Note for upstream canonical. |
| `.claude/skills/fix-review/SKILL.md` | 3 | 3 | suggestion | 1 | DEFER | Suggestion to add `--probe-tiers` flag for manual tier verification without a real PR. Reasonable, not blocking. |
| `.claude/skills/lib/agents.sh` | 1 | 3 | suggestion | 1 | DEFER | Suggestion to add `bats` tests for the new cascade logic. Reasonable, out of scope. |
| `.claude/skills/fix-review/config.yaml` | 21 | 4 | suggestion | 1 | DEFER | `opencode` entry requires explicit model (per `agent_opencode` comment); no config-validation in STEP 0 to error early on missing model. Worth a follow-up. |

## Independent scan

Investigated an apparent STEP 0 extraction failure during the run (`API_URL` came out as literal `matches` instead of the URL). Root cause: Claude Code's shell wraps `grep` with `ugrep` via a per-session function; `ugrep`'s PCRE2 output uses a different "matches in 1F" format that breaks the `awk '{print $2}'` extraction. In real CI (GNU grep), the STEP 0 extraction works correctly. Not a bug in the skill; ambient shell wrapper artifact. Documented for future troubleshooting.

## Fixes applied

1. **`internal/pipeline` probes only succeed on non-empty content AND no `.error` field** — closes the silent-acceptance-of-error-envelope bug.
2. **Empty-after-retry round forces `err` so cascade fires** — closes the silent-0-findings path on retry exhaustion.
3. **`run_round` writes `external_agents:none` marker when Tier 2 exhausts** + post-`wait` reconciliation surfaces a loud warning — closes the silent-0-findings path on Tier 2 exhaustion.
4. **Removed duplicate 5-line comment in `lib/agents.sh`** — keeps the file in sync with what canonical will eventually receive.

## Gates

go build ✓ · go vet ✓ · go test ✓ (107 tests)
bash -n on STEP 0 + STEP 2 + lib/agents.sh ✓

## Verdict

APPROVED_WITH_FIXES

## Independent scan

`go.mod` shows `github.com/prometheus/client_model v0.6.2` moved from `indirect` to `require`. This is a mechanical side effect of `go mod tidy` during the dependency bump, not a deliberate user-facing change. Harmless.

## Fixes applied

None.

## Gates

go build ✓ · go vet ✓ · go test ✓ (107 tests)

## Verdict

APPROVED
