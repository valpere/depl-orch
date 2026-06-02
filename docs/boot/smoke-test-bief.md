# Smoke-Test Brief — depl-orch (Scenario 1, first end-to-end run)

Goal: push one trivial task through BOTH tools to find weak links before any
real code lands. The task is deliberately tiny; what we're testing is the
*plumbing*, not the feature.

**Project dir:** `~/wrk/projects/depl-orch/depl-orch`
**The task:** add `Greet(name string) string` + a passing test, godoc, and a
short README — exercising every link in the chain.

---

## Step 0 — Bootstrap the module (one-time)

```bash
cd ~/wrk/projects/depl-orch/depl-orch
go mod init github.com/valpere/depl-orch
go build ./...   # should now succeed (no files yet = no error)
```

---

## Step 1 — Claude Code: implement (runs on Pro subscription)

In `claude`, give this task:

> Create `greet.go` with an exported function `Greet(name string) string` that
> returns `"Hello, <name>!"`, and `"Hello, stranger!"` when name is empty.
> Add `greet_test.go` with table-driven tests covering both cases. Run
> `go test ./...` and confirm it passes. Then write a one-line changelog to
> `.agents/changes.md` (files touched + why), per AGENTS.md.

**Verify after this step:**
- [ ] `greet.go` and `greet_test.go` exist; `go test ./...` passes.
- [ ] `.agents/changes.md` was written (this proves Claude Code honors the
      handoff protocol from AGENTS.md).

---

## Step 2 — OpenCode: independent verification conveyor

```bash
cd ~/wrk/projects/depl-orch/depl-orch
opencode
```

Then ask the planner:

> Review and verify the Greet implementation. Plan the verification, delegate:
> reviewer checks the diff for bugs/Go idioms and writes `.agents/review.md`;
> tester runs `go test ./...` and writes `.agents/test-report.md`; documenter
> adds a godoc comment to Greet and a short README section, summarizing in
> `.agents/summary.md`.

---

## Step 3 — The three things we're actually testing

Watch for each; these are the likely failure points.

### A. Do the OpenCode roles really CALL tools (not narrate)?
Highest risk: **tester on granite 8b**. A weak model often says "I would run
`go test`..." instead of issuing a real bash call.
- [ ] tester actually executed `go test` (you see real command output, not a
      description of it).
- [ ] coder/documenter actually wrote files (not just printed proposed content).
If tester only narrates → granite is out; swap to a stronger tool-capable model
in `opencode.json` and re-run Step 2.

### B. Do `.agents/*.md` artifacts get written AND read across tools?
- [ ] `.agents/plan.md` written by planner.
- [ ] `.agents/review.md` written by reviewer.
- [ ] `.agents/test-report.md` written by tester.
- [ ] `.agents/summary.md` written by documenter.
- [ ] Spot-check: does `review.md` reference the actual Greet code (proving the
      reviewer read the tree), not generic boilerplate?

### C. Does anything burn paid OpenRouter unexpectedly?
The OpenCode planner (Opus) and reviewer (Gemini) run on **paid** OpenRouter.
- [ ] Note roughly how many OpenRouter calls one `opencode` run cost.
- [ ] Confirm coder/tester/documenter went to Ollama (not OpenRouter) — check
      OpenRouter activity log; only planner+reviewer should appear there.

---

## Step 4 — Report back

Paste or summarize:
1. Which roles invoked real tool calls vs. only narrated.
2. Which `.agents/*.md` files appeared, and whether each later role read the
   previous artifact (e.g. tester's report references reviewer's findings).
3. Any role that failed, errored, or resolved to a wrong/missing model ID.
4. Rough OpenRouter cost of one full run.

We'll use this to fix the weak link (most likely tester) and tune
models/temperatures before putting real orchestrator code through the chain.

---

## Notes
- Don't commit yet — this is throwaway plumbing validation. If you want to keep
  the Greet scaffold, commit `greet.go`/`greet_test.go`/`go.mod` only; `.agents/`
  stays gitignored as decided.
- If Step 1 already feels smooth, the real signal is Step 2 — that's the
  multi-model, multi-tool boundary we haven't exercised yet.
