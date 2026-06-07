---
name: fix-review
description: "depl-orch PR review: 3 Ollama-cloud models parallel → Claude arbiter → .agents/review.md → merge. localhost:11434, no auth. Usage: /fix-review [PR#]"
---

# Skill: /fix-review (depl-orch)

Project-level. Overrides `~/.claude/skills/fix-review/`.

3 Ollama-cloud reviewer models run in parallel via direct REST (`localhost:11434`).
No `Authorization` header — `ollama signin` handles cloud auth transparently.
Claude adjudicates. Result committed to `.agents/review.md` (per AGENTS.md §3).

```
diff ──┬── Round 1 (minimax-m3:cloud)         ┐
       ├── Round 2 (kimi-k2.6:cloud)           ├── aggregate → Arbiter → fix → gates → review.md → merge
       └── Round 3 (devstral-small-2:24b-cloud)┘
```

## RUN COMPLETION CONTRACT

Run is complete only after **both** in order:
1. Step 9 telemetry appended to `telemetry.jsonl`
2. Step 11 final summary printed to user

---

## STEP 0: Resolve PR + load config

```bash
source .claude/skills/lib/rest.sh
CONFIG=".claude/skills/fix-review/config.yaml"

PR_NUMBER="${1:-$(gh pr view --json number --jq '.number' 2>/dev/null)}"
[ -z "$PR_NUMBER" ] && { echo "No PR found. Pass /fix-review <number>"; exit 1; }
BASE_BRANCH=$(gh pr view "$PR_NUMBER" --json baseRefName --jq '.baseRefName')

API_URL=$(grep 'api_url:' "$CONFIG" | awk '{print $2}')
MODEL_R1=$(grep 'round_1:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
MODEL_R2=$(grep 'round_2:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
MODEL_R3=$(grep 'round_3:' "$CONFIG" | sed "s/.*{ model: //;s/ }//")
REST_OLLAMA_TIMEOUT=$(grep 'ollama_timeout_s:' "$CONFIG" | awk '{print $2}')
export REST_OLLAMA_TIMEOUT

RUN_DIR=$(mktemp -d -t fix-review-XXXX)
echo "$RUN_DIR" > /tmp/fixreview-rundir
START_S=$(date +%s)
```

---

## STEP 1: Build prompt

Load project context from `AGENTS.md` (first 150 lines).
Build diff: `git diff $(git merge-base HEAD origin/${BASE_BRANCH})...HEAD`.

```bash
PROJECT_CONTEXT=$(head -150 AGENTS.md)
DIFF=$(git diff "$(git merge-base HEAD "origin/${BASE_BRANCH}")...HEAD")
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

## STEP 2: Fan out 3 rounds in parallel

```bash
run_round() {
  local n="$1" model="$2"
  local payload response
  payload=$(ollama_payload_system "$model" "$REVIEW_SYSTEM" "$PROMPT")
  response=$(rest_post_ollama "$API_URL" "$payload") || response='{"error":"call failed"}'
  printf '%s' "$response" > "$RUN_DIR/round_${n}.raw.json"
  printf '%s' "$model"    > "$RUN_DIR/round_${n}.meta"
}
export -f run_round rest_post rest_post_ollama ollama_payload_system
export API_URL REVIEW_SYSTEM PROMPT RUN_DIR

run_round 1 "$MODEL_R1" &
run_round 2 "$MODEL_R2" &
run_round 3 "$MODEL_R3" &
wait
```

Per-round timeout: `REST_OLLAMA_TIMEOUT` (default 300s in `rest.sh`). A failed round
produces `{"error":"call failed"}` — treated as 0 findings; run still proceeds.

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
    printf '%s' "$content" > "$RUN_DIR/round_${n}.findings.json"
  fi
done
```

---

## STEP 4: Aggregate — dedupe + vote count

```bash
jq -s '
  flatten
  | group_by(.file + ":" + (.line|tostring))
  | map({
      file:     .[0].file,
      line:     .[0].line,
      votes:    length,
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

**Models:** minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud
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
{"ts":"...","pr":N,"run_dir":"...","round":3,"model":"devstral-small-2:24b-cloud","findings":N,"status":"ok|error","ms":N}
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
| Models | minimax-m3:cloud · kimi-k2.6:cloud · devstral-small-2:24b-cloud |
| Findings | total / confirmed / dismissed / deferred |
| Fixes applied | N |
| Gates | pass / fail |
| Merge | squash ✓ / blocked (reason) |

[findings table if any confirmed]
```
