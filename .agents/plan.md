# Plan — M2: model factory + fix-test recovery (bounded, git-rollback)

Source: `docs/depl-orch-requirements.md` §3.2, §5, §6 M2. Builds on M1.
Goal: backend-agnostic model + ONE bounded agentic recovery step (fix-test),
with git snapshot/rollback. Verify: rollback on a deliberately-broken (unfixable)
test; successful fix on a trivially-broken one.

## Hard invariant (unchanged)
`internal/pipeline` imports ONLY stdlib. M2 adds a small `Recoverable` interface
THERE (stdlib only); the agent layer implements it. pipeline never imports agent.

## New dependency (per §5 tech stack)
Eino (CloudWeGo) + eino-ext model components. Pin versions via `go mod tidy`;
verify config struct fields against the pinned release (§7 drift risk).
Concrete API (from context7, 2026-06):
- `model.ToolCallingChatModel` = `BaseChatModel` + `WithTools([]*schema.ToolInfo) (ToolCallingChatModel, error)`; `Generate(ctx, []*schema.Message, ...model.Option) (*schema.Message, error)`.
- `ollama.NewChatModel(ctx, &ollama.ChatModelConfig{BaseURL, Model})`
- `openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey, Model, BaseURL, Temperature *float32, MaxCompletionTokens *int})`
- `claude.NewChatModel(ctx, &claude.Config{APIKey, Model, MaxTokens})`
- tools: `toolutils.InferTool(name, desc, fn)` → `tool.InvokableTool` (`.Info(ctx)`, `.InvokableRun(ctx, argsJSON)`).
- messages: `schema.SystemMessage/UserMessage/AssistantMessage/ToolMessage`; `resp.ToolCalls` (ID, Function.Name, Function.Arguments).

## Files
```
internal/model/factory.go      New(cfg) → model.ToolCallingChatModel, switch by backend
                               (ollama|openai|anthropic) — mirrors NewDeployer shape
internal/model/config.go       env-driven model config (backend, model id, baseURL, key, ...)
internal/agent/tools.go        repo-scoped read_file / write_file built via InferTool,
                               with a path-traversal guard (clean + must stay in repo root)
internal/agent/fixtest.go      FixTest implements pipeline.Recoverable; bounded tool-loop +
                               git snapshot/rollback + rationale to .agents/
internal/agent/loop.go         runToolLoop(ctx, model, tools, msgs, maxIter) — generic bounded
                               generate→dispatch-tool-calls→append loop (no control-flow LLM leak)
internal/agent/git.go          snapshot()/rollback() helpers (git stash-based), repo-scoped
internal/pipeline/recover.go   Recoverable interface + Runner recovery wiring (stdlib only)
*_test.go                      hermetic units: fake ToolCallingChatModel + temp git repo + tmp fs
test/e2e/recover_test.go       //go:build integration — real model fixes a real broken test
```

## Contracts
- `pipeline.Recoverable` (in pipeline, stdlib only):
  `Recover(ctx, st *State, stage string, stageErr error) error`
- `Runner` gains `Recoverer Recoverable` + `MaxRetries int`. On a stage failure:
  if Recoverer set and attempts remain, call `Recover(ctx, st, name, err)`; if it
  returns nil, re-run the stage; if it returns an error, fail (not recoverable).
  Recoverer==nil ⇒ M1 behaviour (fail fast). Deterministic core unchanged when
  Recoverer is nil.
- `FixTest.Recover`: if stage != "test" → return stageErr (not its job). Else it is
  SELF-CONTAINED and SELF-VERIFYING: `git` snapshot → bounded model loop (system
  prompt: "fix the failing Go test by editing SOURCE; NEVER weaken or delete tests;
  use read_file/write_file") → run `go test ./...` itself → if pass: keep changes,
  write rationale to `.agents/fix-test.md`, return nil; if still failing or the
  iteration budget is exhausted: **rollback to snapshot**, return an error. So the
  tree is never left dirty by a broken agent (§2.3), and unfixable fails loudly (§2.4).

## Bounds (§4)
- `maxIterations` per fix-test loop (default 4) — exhaustion = fail, not loop.
- Runner `MaxRetries` for the test stage (default 1 recovery attempt).
- Per-call model timeout via context (the Ollama-hang §7 mitigation, baked in).

## Security (§4)
- Repo-scoped tools only: every path is `filepath.Clean`ed and must remain within
  the repo root (reject `..` escape and absolute-outside); model-generated paths
  are untrusted. No shell (Commander contract). Secrets (API keys) read from env in
  the factory, never logged.

## Test strategy
- model: `New` switch + config validation (unknown backend errors); no live call.
- tools: read/write happy path + path-guard rejection, in a temp dir. Hermetic.
- fix-test: FAKE `ToolCallingChatModel` (implements the eino interface, scripted to
  emit a write_file tool call) + a temp git repo with a trivially-broken test →
  assert fixed + committed-clean; second fake that makes a BAD edit → assert
  rollback (tree restored, error returned). Hermetic — no network/model.
- integration (`-tags integration`): real ollama model fixes a real broken test.

## Definition of done (§6 M2)
Rollback verified on an unfixable broken test; successful fix on a trivial one;
`internal/pipeline` still stdlib-only; gofmt/vet clean; unit tests hermetic + green;
independent OpenCode review (minimax-m3, wrapped in timeout+monitor) before commit;
branch + PR.

## Out of scope
Classifier/cost routing (M3); fix-build, generate-dockerfile, fix-workflow (M4);
deploy + CI conveyor + branch-gating (M5); observability (M6).
