---
name: apply-dreaming
description: "Read the latest depl-orch dreaming report and apply
  high-confidence findings. Routes code-touching changes through
  branch+PR workflow. Direct edits only for project memory and
  .claude/ tooling. Annotates report with [applied YYYY-MM-DD] markers.
  Usage: /apply-dreaming [week|latest]"
user-invocable: true
argument-hint: "[week|latest]"
---

# /apply-dreaming (depl-orch)

Weekly review skill for processing depl-orch dreaming reports from
`.claude/dreaming/reports/`. Walks items interactively.

## When to invoke

- Monday morning after Sunday night's dreaming run.
- Or after manual `.claude/dreaming/dreaming.sh`.
- Or `/apply-dreaming [latest|YYYY-W##]`.

## Steps

### 1. Locate report

```bash
WEEK="${1:-latest}"
DIR=".claude/dreaming/reports"
if [[ "$WEEK" == "latest" ]]; then
  REPORT=$(ls -1t "$DIR"/2026-W*.md 2>/dev/null | head -1)
else
  REPORT="$DIR/$WEEK.md"
fi
```

### 2. Triage walk

Iterate `high → medium → low`. Per item:
- `[a]pply / [s]kip / [v]erify-first / [e]vidence / [q]uit`

For `low`: skip silently unless user opted in.

### 3. Apply per category

#### `update-memory` — `~/.claude/projects/.../memory/<file>.md`

Local-only, gitignored. Edit directly. No commit needed.

#### `update-rules` / `update-claude-md`

1. `git switch -c docs-dreaming-W##`
2. Edit `.claude/context-essentials.md` or `CLAUDE.md`.
3. Commit, push, `gh pr create`.

#### `code-change` / `add-test`

1. `git switch -c fix-dreaming-W##`
2. Implement change.
3. `go build ./... && go vet ./... && go test ./...`
4. Commit, push, PR.
5. Run `/fix-review <num>`.

#### `forget-memory`

1. Backup to `/tmp/dreaming-W##-depl-orch-backup-HHMM/`.
2. Confirm, then `rm <file>`.

### 4. Annotate report

```markdown
> [applied YYYY-MM-DD: <action>; commit <sha>; PR <num>]
```

## Constraints

- **Never push directly to main** — branch + PR always.
- **Never auto-apply low confidence** without explicit request.
- **Always run `go build ./... && go test ./...`** after code changes.
- **Always cite report section** in commit messages.
