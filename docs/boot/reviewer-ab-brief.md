# Claude Code Brief — Drop OpenRouter + Reviewer A/B (Scenario 1b, final)

Two goals:
1. Remove all OpenRouter dependency. The stack now runs entirely on what's
   already paid for: **Claude Pro subscription** (Claude Code = driver/planner/
   coder) + **Ollama Pro** (delegated reviewer/tester/documenter, cloud models).
2. Pick the reviewer model by data: A/B test `minimax-m3:cloud` (brand new,
   huge context, MiniMax family) against `kimi-k2.6:cloud` (proven) on a real
   review, then keep the winner.

**Project dir:** `~/wrk/projects/depl-orch/depl-orch`

## Why reviewer can be any non-Claude Ollama model

Independence rule = reviewer must differ from whoever WROTE the code. Claude
Code writes the code (Claude family), so any Ollama model (MiniMax, Kimi, GLM,
Qwen) satisfies independence. OpenRouter/Gemini is no longer needed.

## Task 1 — Edit `opencode.json` (remove OpenRouter, fix reviewer)

- DELETE the `planner` agent that used `openrouter/anthropic/claude-opus-4`.
  In headless mode the planner IS Claude Code; OpenCode only runs delegated
  worker roles. (Keep planner only if you still use the OpenCode TUI standalone;
  if so, repoint it to an Ollama model, not OpenRouter.)
- DELETE the `reviewer` line using `openrouter/google/gemini-2.5-pro`.
- Set reviewer to a variable we'll A/B. Start with minimax-m3:

```jsonc
"reviewer": {
  "description": "Independent review: bugs, security, Go idioms. Read-only. Non-Claude family (reviews code Claude Code wrote).",
  "mode": "subagent",
  "model": "ollama/minimax-m3:cloud",
  "temperature": 0.3,
  "tools": { "write": false, "edit": false, "bash": false }
}
```

- In `provider.ollama.models`, add BOTH candidates so we can switch by `--model`:
```jsonc
"minimax-m3:cloud": { "tools": true },
"kimi-k2.6:cloud":  { "tools": true }
```
- coder/tester/documenter stay on their Ollama cloud models unchanged.

## Task 2 — Clean `.env`

`OPENROUTER_API_KEY` is no longer used by any role. Remove it from `.env`
(leave `OLLAMA_URL`). Confirm `.env` stays gitignored. Don't print key values.

## Task 3 — Confirm both models are reachable

```bash
ollama signin                 # ensure Pro cloud auth
ollama list | grep -E 'minimax-m3|kimi-k2.6'   # confirm both cloud models resolve
opencode models | grep -E 'minimax-m3|kimi-k2.6'
```
If `minimax-m3:cloud` doesn't resolve yet (it was published ~today), note it and
proceed with kimi only; we'll add M3 once it's pullable on your account.

## Task 4 — Reviewer A/B on the Greet task

Ensure `greet.go` + `greet_test.go` exist (reuse smoke-test scaffold). Produce
the diff once, then run the SAME review prompt against each model:

```bash
git --no-pager diff > .agents/_diff.patch

# Candidate A: MiniMax M3
opencode run -q --agent reviewer --model ollama/minimax-m3:cloud \
  "Read .agents/_diff.patch and the referenced source files. Review for bugs, \
   security, and Go idioms. Write findings to .agents/review-m3.md. Do not edit code."

# Candidate B: Kimi K2.6
opencode run -q --agent reviewer --model ollama/kimi-k2.6:cloud \
  "Read .agents/_diff.patch and the referenced source files. Review for bugs, \
   security, and Go idioms. Write findings to .agents/review-kimi.md. Do not edit code."
```

(Note: writing to different output files lets us compare side by side.)

## Task 5 — Compare and decide

Read both `.agents/review-m3.md` and `.agents/review-kimi.md`. Score each on:

| Criterion                          | M3 | Kimi |
|------------------------------------|----|----|
| Did it actually invoke tools (read the files) vs. narrate? | ?  | ?  |
| Did it finish without Ollama Cloud timeout/hang?           | ?  | ?  |
| Does the review reference the REAL Greet code (not generic)? | ?  | ?  |
| Substantive findings (real issues, idiomatic Go) vs. fluff? | ?  | ?  |
| Rough latency (cold start to result)                        | ?  | ?  |

Pick the winner, set it as the permanent `reviewer` model in `opencode.json`,
and delete the loser from the prompt flow (keep it listed in provider.models as
a fallback). Update the `delegate-opencode` skill's review command to drop the
explicit `--model` (so it uses the chosen default) and write to `.agents/review.md`.

## Task 6 — End-to-end headless check (no manual switching)

In one `claude` session, ask: *"Get an independent review of the Greet code, run
the tests, and generate docs."* Confirm:
- [ ] Claude Code invoked the skill and called `opencode run` itself.
- [ ] `.agents/review.md` (winner), `.agents/test-report.md`, `.agents/summary.md` all written.
- [ ] tester's report shows a REAL `go test` run, not a description.
- [ ] You never touched the OpenCode terminal.
- [ ] Nothing hit OpenRouter (there's no key anymore — confirm no errors trying).

## Stop point
Show me the A/B table, both review files, and the final `opencode.json` reviewer
line before committing. Flag any model that timed out or only narrated.

## Final expected setup

| Role            | Tool               | Model                         | Billing        |
|-----------------|--------------------|-------------------------------|----------------|
| planner + coder | Claude Code        | Claude (Opus/Sonnet)          | Pro subscription |
| reviewer        | OpenCode headless  | A/B winner (M3 or Kimi)       | Ollama Pro     |
| tester          | OpenCode headless  | granite3.3:8b-cloud           | Ollama Pro     |
| documenter      | OpenCode headless  | qwen2.5-coder:7b-cloud        | Ollama Pro     |

Two fixed-price subscriptions, zero per-token spend, review independence intact.
