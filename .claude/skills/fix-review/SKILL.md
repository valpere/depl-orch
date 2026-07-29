---
name: fix-review
description: "depl-orch PR review: 3 Ollama-cloud models parallel → Claude arbiter → .agents/review.md → merge. Optional tier-2 (external agent CLIs) and tier-3 (Ollama local) failover when configured. localhost:11434. Usage: /fix-review [PR#]"
---

# Skill: /fix-review (depl-orch)

Project-level. Overrides `~/.claude/skills/fix-review/`.

3 Ollama-cloud reviewer models run in parallel via direct REST (`localhost:11434`).
No `Authorization` header — `ollama signin` handles cloud auth transparently.
Claude adjudicates. Result committed to `.agents/review.md` (per AGENTS.md §3).

```
diff ──┬── Round 1 (minimax-m3:cloud)         ┐
       ├── Round 2 (kimi-k2.6:cloud)           ├── aggregate → Arbiter → fix → gates → review.md → merge
       └── Round 3 (gemma4:31b-cloud)┘
                          ↓ on cloud round failure
              Tier 2: external_agents (cursor-agent | omp | codex | opencode | kilo)
                          ↓ on Tier 2 failure
              Tier 3: ollama_local (qwen2.5-coder:7b | granite3.3:8b | qwen3-coder:30b)
                          ↓ on Tier 3 failure
              hard failure (round produces 0 findings, surfaced in Step 11)
```

## RUN COMPLETION CONTRACT

Run is complete only after **both** in order:
1. Step 9 telemetry appended to `telemetry.jsonl`
2. Step 11 final summary printed to user

---

## STEP 0: Resolve PR + load config + probe + tiers

```bash
source .claude/skills/lib/rest.sh
source .claude/skills/lib/agents.sh        # try_external_agents + run_external_agent
CONFIG=".claude/skills/fix-review/config.yaml"

PR_NUMBER="${1:-$(gh pr view --json number --jq '.number' 2>/dev/null)}"
[ -z "$PR_NUMBER" ] && { echo "No PR found. Pass /fix-review <number>"; exit 1; }
BASE_BRANCH=$(gh pr view "$PR_NUMBER" --json baseRefName --jq '.baseRefName')

API_URL=$(yq -r '.ollama_api_url // .api_url // ""' "$CONFIG" 2>/dev/null \
          || grep 'ollama_api_url\|api_url:' "$CONFIG" | head -1 | awk '{print $2}')
[ -z "$API_URL" ] && { echo "no ollama_api_url/api_url in $CONFIG"; exit 1; }
MODEL_R1=$(yq -r '.reviewers.round_1.model' "$CONFIG" 2>/dev/null \
           || grep 'round_1:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
MODEL_R2=$(yq -r '.reviewers.round_2.model' "$CONFIG" 2>/dev/null \
           || grep 'round_2:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
MODEL_R3=$(yq -r '.reviewers.round_3.model' "$CONFIG" 2>/dev/null \
           || grep 'round_3:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
REST_OLLAMA_TIMEOUT=$(yq -r '.ollama_timeout_s' "$CONFIG" 2>/dev/null \
                      || grep 'ollama_timeout_s:' "$CONFIG" | awk '{print $2}')
export REST_OLLAMA_TIMEOUT

# Active config — read from STEP 2/3 onwards via these (not the raw MODEL_R*).
ACTIVE_API_URL="$API_URL"
ACTIVE_KEY=""                                # depl-orch uses ollama signin (no key)
ACTIVE_MODELS=("$MODEL_R1" "$MODEL_R2" "$MODEL_R3")
FAILOVER_TIER=""                              # "" | "external_agents" | "ollama_local" | "cloud_unavailable"
FAILOVER_REASON=""

# Tier existence checks (driven by config presence).
EXTERNAL_AGENTS_EXIST="no"
if command -v yq >/dev/null 2>&1; then
  yq -e '.reviewers.external_agents' "$CONFIG" >/dev/null 2>&1 && EXTERNAL_AGENTS_EXIST="yes"
fi

LOCAL_TIER_EXISTS="no"
if command -v yq >/dev/null 2>&1; then
  _lm1=$(yq -r '.reviewers.ollama_local.round_1.model // ""' "$CONFIG" 2>/dev/null)
  _lm2=$(yq -r '.reviewers.ollama_local.round_2.model // ""' "$CONFIG" 2>/dev/null)
  _lm3=$(yq -r '.reviewers.ollama_local.round_3.model // ""' "$CONFIG" 2>/dev/null)
  [ -n "$_lm1" ] && [ -n "$_lm2" ] && [ -n "$_lm3" ] && LOCAL_TIER_EXISTS="yes"
fi

# Provider probe — call the local daemon with the smallest possible payload
# (no Authorization header — depl-orch's daemon uses device-auth via
# `ollama signin`, not OLLAMA_API_KEY; canonical's probe_provider uses
# Bearer auth which would be wrong here).
deplorch_probe() {
  local url="$1" model="$2"
  local payload resp
  payload=$(jq -n --arg m "$model" \
    '{model:$m, messages:[{role:"user",content:"OK"}], stream:false, max_tokens:3}')
  resp=$(REST_TIMEOUT=10 rest_post_ollama "$url" "$payload") || resp=""
  printf '%s' "$resp" | jq -e '.message.content // empty' >/dev/null 2>&1
}

CLOUD_KNOWN_BAD="no"
if ! deplorch_probe "$ACTIVE_API_URL" "$MODEL_R2" 2>&1; then
  if [ "$EXTERNAL_AGENTS_EXIST" = "yes" ] || [ "$LOCAL_TIER_EXISTS" = "yes" ]; then
    echo "⚠️  FAILOVER: Ollama cloud daemon unreachable — routing rounds through failover tiers"
    CLOUD_KNOWN_BAD="yes"
    FAILOVER_REASON="daemon probe failed"
  else
    echo "⚠️  WARNING: Ollama cloud daemon unreachable and no failover tier configured"
    FAILOVER_TIER="cloud_unavailable"
    FAILOVER_REASON="daemon down, no failover tier"
  fi
fi

echo "→ Tier 0 (cloud): probe=$([ "$CLOUD_KNOWN_BAD" = "yes" ] && echo BAD || echo ok)   " \
     "Tier 2 (external_agents): $EXTERNAL_AGENTS_EXIST   " \
     "Tier 3 (ollama_local): $LOCAL_TIER_EXISTS"

RUN_DIR=$(mktemp -d -t fix-review-XXXX)
echo "$RUN_DIR" > /tmp/fixreview-rundir
START_S=$(date +%s)
```

---

## STEP 1: Build prompt

Load project context from `AGENTS.md` (first 150 lines).
Build diff: `gh pr diff "${PR_NUMBER}"`.

```bash
PROJECT_CONTEXT=$(head -150 AGENTS.md)
DIFF=$(gh pr diff "${PR_NUMBER}")
printf '%s' "$DIFF" > "$RUN_DIR/diff.txt"

REVIEW_SYSTEM="You are a senior code reviewer. Your entire response MUST be a raw JSON array — nothing else. Start with [ and end with ]. No prose, no markdown fences. If no issues: []"

PROMPT="Review this Go codebase diff using the Code Review Pyramid (layers 1-4; skip layer 5 — style/formatting).

== Project context ==
${PROJECT_CONTEXT}
== End project context ==

Return ONLY a raw JSON array. Each item must have exactly:
  \"file\"     — relative path (string)
  \"line\"     — line number on the + side of the diff (integer)
  \"layer\"    — 1-4 (integer)
  \"severity\" — \"error\" | \"warning\" | \"suggestion\" (string)
  \"body\"     — description of the issue and how to fix it (string)

Layer guide:
  1 (base) — API/Architecture: layer violations (pipeline must import only stdlib),
              contract drift, banned patterns, hidden coupling.
  2        — Implementation: bugs, nil deref, error handling, resource leaks,
              race conditions, security holes, missing context propagation.
  3        — Tests/Docs: missing tests for critical paths, undocumented public APIs.
  4        — Minor: non-blocking improvements.
  5 (skip) — Style/formatting — DO NOT FLAG.

Severity guide:
  error      — must fix before merge
  warning    — should fix
  suggestion — nice to have

DO NOT flag code not present in this diff.
DO NOT propose architectural rewrites.

Git diff:
---
${DIFF}
---"

printf '%s' "$PROMPT" > "$RUN_DIR/prompt.txt"
```

---

## STEP 2: Fan out 3 rounds in parallel (with cascade)

```bash
run_round() {
  local n="$1" model="$2"
  local r_start r_end payload response err content
  r_start=$(python3 -c "import time;print(int(time.time()*1000))" 2>/dev/null || echo $(($(date +%s) * 1000)))

  if [ "$CLOUD_KNOWN_BAD" = "yes" ]; then
    err="cloud known unavailable (probe failed)"
    response='{"error":"cloud known unavailable"}'
  else
    payload=$(ollama_payload_system "$model" "$REVIEW_SYSTEM" "$PROMPT")
    response=$(rest_post_ollama "$API_URL" "$payload") || response='{"error":"call failed"}'
    err=$(printf '%s' "$response" | jq -r '.error.message // .error // empty' 2>/dev/null)
    [ -z "$err" ] && {
      content=$(ollama_content "$response")
      if [ -z "$(printf '%s' "$content" | tr -d '[:space:]')" ]; then
        echo "warn: round ${n} (${model}) returned empty content — retrying once" >&2
        response=$(rest_post_ollama "$API_URL" "$payload") || response='{"error":"call failed"}'
        err=$(printf '%s' "$response" | jq -r '.error.message // .error // empty' 2>/dev/null)
      fi
    }
  fi

  # Tier 2 — external agent CLIs (independent subscriptions).
  if [ -n "$err" ] && [ "$FAILOVER_TIER" != "cloud_unavailable" ] && [ "$EXTERNAL_AGENTS_EXIST" = "yes" ]; then
    echo "warn: round ${n} error (${err}) — trying external_agents" >&2
    local prompt_file; prompt_file=$(mktemp)
    trap "rm -f '$prompt_file'" RETURN
    printf '%s\n\n%s' "$REVIEW_SYSTEM" "$PROMPT" > "$prompt_file"
    if try_external_agents "$n" "$prompt_file" "$CONFIG" "$RUN_DIR"; then
      r_end=$(python3 -c "import time;print(int(time.time()*1000))" 2>/dev/null || echo $(($(date +%s) * 1000)))
      local winning_tool; winning_tool=$(head -1 "$RUN_DIR/round_${n}.meta")
      printf '%s\n%s' "$winning_tool" "$((r_end - r_start))" > "$RUN_DIR/round_${n}.meta"
      return 0
    fi
    err="all external_agents failed"
  fi

  # Tier 3 — Ollama local (fully offline, no key).
  if [ -n "$err" ] && [ "$FAILOVER_TIER" != "cloud_unavailable" ] && [ "$LOCAL_TIER_EXISTS" = "yes" ]; then
    local local_model
    local_model=$(yq -r ".reviewers.ollama_local.round_${n}.model // \"\"" "$CONFIG" 2>/dev/null)
    echo "warn: round ${n} error (${err}) — trying Ollama local (${local_model})" >&2
    payload=$(ollama_payload_system "$local_model" "$REVIEW_SYSTEM" "$PROMPT")
    response=$(rest_post_ollama "$API_URL" "$payload") || response='{"error":"ollama local failover failed"}'
    model="$local_model"
    printf '%s' "$local_model" > "$RUN_DIR/round_${n}.failover"
  fi

  r_end=$(python3 -c "import time;print(int(time.time()*1000))" 2>/dev/null || echo $(($(date +%s) * 1000)))
  printf '%s' "$response"   > "$RUN_DIR/round_${n}.raw.json"
  printf '%s\n%s' "$model"  "$((r_end - r_start))" > "$RUN_DIR/round_${n}.meta"
}
export -f run_round rest_post rest_post_ollama ollama_payload ollama_payload_system \
            ollama_content try_external_agents run_external_agent \
            agent_cursor_agent agent_omp agent_codex agent_opencode agent_kilo
export API_URL REVIEW_SYSTEM PROMPT RUN_DIR \
       CLOUD_KNOWN_BAD FAILOVER_TIER EXTERNAL_AGENTS_EXIST LOCAL_TIER_EXISTS CONFIG

run_round 1 "$MODEL_R1" &
run_round 2 "$MODEL_R2" &
run_round 3 "$MODEL_R3" &
wait

# Reconcile per-round failover markers into FAILOVER_TIER for Step 11 reporting.
# (Subshells can't update parent vars; each round's Tier 2/3 success wrote
# $RUN_DIR/round_${n}.failover. Prefer reporting external_agents if any round
# used it — Step 11 still reads each marker individually for per-round detail.)
if [ "$FAILOVER_TIER" = "" ] && ls "$RUN_DIR"/round_*.failover >/dev/null 2>&1; then
  if grep -l '^external_agents:' "$RUN_DIR"/round_*.failover >/dev/null 2>&1; then
    FAILOVER_TIER="external_agents"
  else
    FAILOVER_TIER="ollama_local"
  fi
  FAILOVER_REASON="per-round error mid-review (see round_*.failover)"
fi
```

Per-round timeout: `REST_OLLAMA_TIMEOUT` (default 600s). A round that fails
the cloud call cascades through `external_agents` (Tier 2), then `ollama_local`
(Tier 3). Only when **all three tiers** fail for a round does it produce
`{"error":"call failed"}` and contribute 0 findings to the aggregate.

---

## STEP 3: Parse each response → findings array

```bash
for n in 1 2 3; do
  raw=$(cat "$RUN_DIR/round_${n}.raw.json")
  content=$(ollama_content "$raw")
  # Strip code fences if model ignored instructions
  content=$(printf '%s' "$content" | sed -E 's/^```(json)?[[:space:]]*//; s/[[:space:]]*```$//')
  if ! printf '%s' "$content" | jq -e 'type == "array"' >/dev/null 2>&1; then
    printf 'warn: round %s not a JSON array — 0 findings\n' "$n" >&2
    echo "[]" > "$RUN_DIR/round_${n}.findings.json"
  else
    # Tag each finding with the model name so Step 4 can count unique models per location.
    model=$(cat "$RUN_DIR/round_${n}.meta")
    printf '%s' "$content" | jq --arg m "$model" 'map(. + {model: $m})' \
      > "$RUN_DIR/round_${n}.findings.json"
  fi
done
```

---

## STEP 4: Aggregate — dedupe + vote count

`votes` = number of **distinct models** that flagged the same (file, line).
A model returning multiple findings for the same line still counts as 1 vote.

```bash
jq -s '
  flatten
  | group_by(.file + ":" + (.line|tostring))
  | map({
      file:     .[0].file,
      line:     .[0].line,
      votes:    ([.[] | .model] | unique | length),
      models:   ([.[] | .model] | unique),
      body:     ([.[] | .body] | sort_by(length) | last),
      severity: ([.[] | .severity] | if any(.=="error") then "error"
                                     elif any(.=="warning") then "warning"
                                     else "suggestion" end),
      layer:    ([.[] | .layer] | min)
    })
  | sort_by(.layer,
      (if .severity=="error" then 0 elif .severity=="warning" then 1 else 2 end),
      -.votes)
' "$RUN_DIR"/round_*.findings.json > "$RUN_DIR/aggregated.json"
```

With 3 rounds: 3/3 = high confidence, 2/3 = medium, 1/3 = low.

---

## STEP 5: Arbiter (Claude)

Read `$RUN_DIR/aggregated.json`. For each finding, rule:

| Ruling    | When |
|-----------|------|
| CONFIRM   | Real issue. Default for `votes ≥ 2`. |
| DISMISS   | False positive, contradicts project rules, or layer-5 noise. Default for `votes == 1` unless obviously real. |
| ESCALATE  | Real but more severe than tagged. |
| DEFER     | Real but out of scope for this PR. Note in review.md, don't fix. |

**Vote count is a prior, not a verdict.** A 1-vote finding pointing to a real
security hole or layering violation should be CONFIRMED. A 3-vote finding that
contradicts `AGENTS.md` load-bearing invariants should be DISMISSED with reason.

**Key invariants to enforce (from AGENTS.md):**
- `internal/pipeline` imports stdlib only — any import of LLM/agent packages is a layer-1 error.
- `Recoverer == nil` ⇒ exact M1 deterministic behaviour preserved.
- Secrets never logged, API keys read from env only.
- Path-traversal guard on all repo-scoped file access.

**Independent scan:** also walk the full diff once and flag anything all 3 models
missed (errors and warnings only — don't add suggestions).

---

## STEP 6: Apply confirmed fixes

For each CONFIRM/ESCALATE finding, apply the fix via Edit tool.
Rules:
- Never weaken or delete tests.
- Never modify test files (`*_test.go`) unless the finding is in a test file.
- Minimal change that addresses the finding.

After all edits: run gates (Step 7). Revert any edit that breaks gates.

---

## STEP 7: Run gates

```bash
go build ./... && go vet ./... && go test ./...
```

Gates must pass before proceeding. If gates fail after a fix attempt, revert
that fix (Edit back to original), mark finding as DEFERRED in review.md, continue.

---

## STEP 8: Write `.agents/review.md` + commit

Write `.agents/review.md`:

```markdown
# PR #N review — YYYY-MM-DD

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · gemma4:31b-cloud
**Rounds:** 3 parallel via localhost:11434
**Arbiter:** Claude

## Findings

| File | Line | Layer | Sev | Votes | Ruling | Notes |
|------|------|-------|-----|-------|--------|-------|
...

## Fixes applied
...

## Gates
go build ✓ · go vet ✓ · go test ✓ (N tests)

## Verdict
APPROVED | APPROVED_WITH_FIXES | BLOCKED
```

Then commit (fixes first, then review.md):

```bash
# If fixes were applied:
git add <changed source files>
git commit -m "fix(review): apply /fix-review findings for PR #${PR_NUMBER}

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"

# Always commit review.md:
git add .agents/review.md
git commit -m "docs: /fix-review notes for PR #${PR_NUMBER}

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"

git push
```

---

## STEP 9: Telemetry

Append to `$TELEMETRY_FILE` (from config, default `.claude/skills/fix-review/telemetry.jsonl`):

One row per model round + one arbiter row:
```jsonl
{"ts":"...","pr":N,"run_dir":"...","round":1,"model":"minimax-m3:cloud","findings":N,"status":"ok|error","ms":N}
{"ts":"...","pr":N,"run_dir":"...","round":2,"model":"kimi-k2.6:cloud","findings":N,"status":"ok|error","ms":N}
{"ts":"...","pr":N,"run_dir":"...","round":3,"model":"gemma4:31b-cloud","findings":N,"status":"ok|error","ms":N}
{"ts":"...","pr":N,"run_dir":"...","role":"arbiter","model":"claude","confirmed":N,"dismissed":N,"deferred":N,"fixes_applied":N,"merged":true|false}
```

---

## STEP 10: Merge PR

```bash
gh pr checks "$PR_NUMBER"   # must be all passing or non-required failures only
gh pr merge "$PR_NUMBER" --squash --delete-branch
```

If checks are failing: print a warning and ask the user once before merging.
If the PR has conflicts: stop and report — do NOT force.

---

## STEP 11: Final summary (mandatory)

Print to user:

```
### /fix-review PR #N — done

| | |
|---|---|
| Models | minimax-m3:cloud · kimi-k2.6:cloud · gemma4:31b-cloud |
| Findings | total / confirmed / dismissed / deferred |
| Fixes applied | N |
| Gates | pass / fail |
| Merge | squash ✓ / blocked (reason) |

[findings table if any confirmed]
```

**Failover reporting (mandatory when any round used a fallback tier):**
After the summary block, if `$FAILOVER_TIER` is non-empty **or** any
`$RUN_DIR/round_*.failover` file exists, append a `### ⚠️ Provider failover`
section:

```
### ⚠️ Provider failover

Tier used: ${FAILOVER_TIER}   # external_agents | ollama_local | cloud_unavailable
Reason: ${FAILOVER_REASON}
Primary models: ${MODEL_R1} · ${MODEL_R2} · ${MODEL_R3}
Round-by-round:
- round 1 → <details from round_1.failover if present>
- round 2 → <details from round_2.failover if present>
- round 3 → <details from round_3.failover if present>
```

Marker-file conventions (set by STEP 2):
- `external_agents:<tool>` → "fell back to external agent `<tool>`"
- bare local model name → "fell back to Ollama local `<model>`"

When `FAILOVER_TIER=cloud_unavailable`, the PR was NOT reviewed (0 findings);
do NOT merge blindly — print the warning, ask the user to check
`ollama serve` and retry.
