# depl-orch — Requirements & Design

A deterministic deployment conveyor in Go with **bounded** agentic recovery.
The pipeline (build → test → docker → push → deploy) is plain Go and reproducible
in CI. LLM "thinking" is confined to individual recovery/generation steps, never
to control flow.

> Status: M0/M1 skeleton exists (pipeline core, deterministic stages, model
> factory, fix-test recovery, repo-scoped tools). This doc defines the full
> target and the path from skeleton to v1.

---

## 1. Purpose & non-goals

**Purpose.** Take a git repository to a deployed artifact through a fixed,
auditable sequence of stages, automatically attempting narrow LLM-backed fixes
when a deterministic stage fails (e.g. a failing test, a missing/broken
Dockerfile), with git-based rollback and a hard retry budget.

**Explicit non-goals (v1).**
- Not a general-purpose autonomous agent. It does not "decide what to build."
- Not a replacement for the interactive dev layer (Claude Code + OpenCode);
  that layer produces code, this conveyor ships it.
- No free-form multi-agent orchestration in the control flow. Control flow is
  code. (Multi-agent belongs to the interactive layer.)
- No TypeScript variant in v1 (Go only; TS reconsidered later if needed).

---

## 2. Design principles (non-negotiable)

1. **Control flow is Go, not an LLM.** The stage sequence and retry logic are
   ordinary code. An LLM is invoked only *inside* a stage that explicitly needs
   judgment (fix a test, generate a Dockerfile). This is what makes CI runs
   reproducible.
2. **State lives in the git working tree + on-disk artifacts**, never passed as
   another component's chat context. Agentic steps read files; they don't
   inherit a conversation. (Mirrors the interactive layer's `.agents/*` pattern.)
3. **Every agentic edit is reversible.** Snapshot before, rollback on failure or
   on a still-failing re-run. The tree is never left dirty by a broken agent.
4. **LLM usage is bounded.** Max iterations per agent, max retries per stage,
   and a per-run budget cap. An agentic step that can't fix in N tries fails
   loudly rather than looping.
5. **Backend-agnostic.** The same binary runs on Ollama Pro, any OpenAI-compatible
   endpoint, or Anthropic — selected by config, not code. (Matches the two
   environments: home = Ollama Pro; org = whatever the org provides.)
6. **Cost flows downhill.** A cheap classifier decides whether a step needs a
   strong fixer at all; default to the cheapest model that can do the job.

---

## 3. Functional requirements

### 3.1 Pipeline stages (deterministic)
- **build** — `go build ./...`; fail with captured output.
- **test** — `go test ./...`; on failure, capture output into State and signal
  recovery.
- **docker-build** — build image from a Dockerfile to a tag.
- **docker-push** — push to the configured registry.
- **deploy** — apply the pushed image to a deploy target via the `Deployer`
  interface. Target is chosen per run by config/flag (`DEPLOYER_TARGET`), one at
  a time: `compose` or `k8s`. The orchestrator executes the chosen target; it
  does not infer it. Two implementations in v1:
  - `ComposeDeployer` — `docker compose -f <file> up -d` with the new image tag;
    rollback = restore previous tag and re-up.
  - `K8sDeployer` — `helm upgrade <release>` (preferred over raw `kubectl apply`
    because `helm rollback` gives one-command, versioned rollback that matches
    the project-wide "every step reversible" rule).
- Stages are ordered, each idempotent where possible, each logs start/end/elapsed.

### 3.2 Agentic recovery / generation steps (bounded, optional per run)
- **fix-test** — given failing `go test` output, read the relevant files and
  apply the minimal fix; must not weaken or delete tests. (Exists in skeleton.)
- **fix-build** — given a compile error, apply the minimal fix.
- **generate-dockerfile** — if no Dockerfile exists, generate one for the detected
  Go project; idempotent (no-op if present and valid).
- **fix-workflow** — repair a broken GitHub Actions workflow YAML.
- Each step: repo-scoped file tools only, git snapshot + rollback, bounded
  iterations, writes a short rationale to `.agents/` for auditability.

### 3.3 Cost-aware routing
- A **classifier** (cheap, Haiku-class or a small Ollama model) inspects a failure
  and decides: trivial (let the cheap fixer try) vs. complex (escalate to a strong
  fixer) vs. not-LLM-fixable (fail fast). The classifier is a node *inside* the
  deterministic flow, not the flow itself.

### 3.4 CI integration
- Runs as a single step in GitHub Actions; exit code reflects pipeline result.
- Agentic recovery is **gated by branch**: enabled on feature branches, disabled
  (`MAX_RETRIES=0`) on `main`/release. Agent-applied changes surface as a diff /
  PR, never a silent push to `main`.

### 3.5 Observability
- Per-stage structured logs (JSON), elapsed time, attempt count.
- Eino callbacks (OnStart/OnEnd/OnError) feed token usage + cost per agentic step
  to Prometheus; Grafana dashboard for run duration, retry rate, cost per run.
  (Reuse the existing Prometheus/Grafana stack.)

---

## 4. Non-functional requirements

- **Determinism:** identical inputs + agentic-off ⇒ identical stage sequence and
  result. Agentic-on is best-effort and always bounded.
- **Security:** secrets never logged; repo-scoped file tools with path-traversal
  guard (model-generated paths are untrusted); no agentic edits on protected
  branches; `.env` and credentials never read into prompts.
- **Reliability:** safe retries; a crashed agent rolls back cleanly; partial
  pushes are detectable.
- **Configurability:** all backend/model/budget/branch-gating via env or a config
  file; no rebuild to change provider.
- **Portability:** runs on a GH Actions runner and on a developer machine
  unchanged.

---

## 5. Architecture

```
cmd/deployer/main.go         wiring: config → model factory → pipeline → run
internal/pipeline/           DETERMINISTIC core
  pipeline.go                Stage, State, Recoverable, runner + retry/rollback
  stages.go                  build, test, docker-build, docker-push
  deploy.go                  Deployer interface + factory (compose | k8s) + deploy Stage
internal/model/factory.go    backend-agnostic ToolCallingChatModel (Eino)
internal/agent/              BOUNDED LLM steps
  fixtest.go                 fix-test recovery (+ git snapshot/rollback)
  fixbuild.go                fix-build recovery
  dockerfile.go              generate/fix Dockerfile
  classifier.go              cheap triage: trivial / complex / not-fixable
  tools.go                   repo-scoped read_file / write_file (guarded)
internal/obs/                Eino callbacks → metrics
```

**Three places, one pattern.** Dependency selection is uniform across the
codebase — an interface plus a config-chosen implementation: the model factory
(ollama | openai-compat | anthropic) and the Deployer factory (compose | k8s).
Same shape, so a reader recognizes it immediately and a new target/backend is a
small, isolated addition.

**Layering rule:** `pipeline` never imports an LLM. `agent` implements
`pipeline.Recoverable` (and similar small interfaces) so the deterministic core
calls into agentic steps through a narrow contract, not the reverse.

**Tech stack:** Go 1.26; Eino (CloudWeGo) as the agent/tool-calling substrate
(ADK `ChatModelAgent`, `Runner`, `InferTool`, callbacks); model components from
`eino-ext` (ollama / openai / claude). git + docker shelled out. Versions pinned
via `go mod tidy`; verify `eino-ext` config struct field names against the pinned
release (they drift between versions).

**Deployer contract (v1):**

```go
// internal/pipeline/deploy.go
type Deployer interface {
    Name() string
    Deploy(ctx context.Context, st *State) error   // apply image st.ImageTag
    Rollback(ctx context.Context, st *State) error  // revert to previous version
}

// NewDeployer selects the implementation by config, mirroring model.New.
func NewDeployer(target string, cfg DeployConfig) (Deployer, error) {
    switch target {            // DEPLOYER_TARGET
    case "compose": return &ComposeDeployer{...}, nil
    case "k8s":     return &K8sDeployer{...}, nil   // helm-backed
    default:        return nil, fmt.Errorf("unknown deploy target %q", target)
    }
}
```

The deploy `Stage` wraps the chosen `Deployer`; on a later-stage failure the
pipeline can call `Rollback` just as it rolls back agentic edits via git.

---

## 6. Phased roadmap

Each milestone is independently valuable and independently testable. Do not start
a milestone until the previous one is green.

- **M0 — Scaffold.** Module, layout, `go build`/`go test` run. *(done)*
- **M1 — Deterministic pipeline, no LLM.** build/test/docker-build/docker-push
  green end-to-end on a sample repo with a hand-written Dockerfile. This is the
  reproducible spine; prove it before adding any model. *(skeleton present;
  finish + test)*
- **M2 — Model factory + fix-test with rollback.** Backend-agnostic model;
  one bounded agentic recovery step; verify rollback on a deliberately broken
  test, and a successful fix on a trivially broken one.
- **M3 — Cost-aware classifier.** Cheap triage node selects fixer tier or
  fails fast. Verify it doesn't escalate trivial cases and doesn't loop on
  unfixable ones.
- **M4 — More agentic steps.** fix-build, generate-dockerfile, fix-workflow,
  each following the fix-test pattern (scoped tools, rollback, bounded, audited).
- **M5 — Deploy stage + GitHub Actions.** Real deploy target wired; conveyor runs
  as a CI step; branch-gating for agentic recovery; agent changes → PR.
- **M6 — Observability.** Eino callbacks → Prometheus → Grafana; cost/retry/
  duration per run.

---

## 7. Risks & mitigations

- **LLM toolchain instability** (Ollama Cloud timeouts/hangs observed in the
  interactive layer). → Per-call timeout + bounded retries; classifier can mark
  "not-fixable" and fail fast; fixer backend swappable via config.
- **Non-determinism leaking into control flow.** → Hard architectural rule:
  `pipeline` never imports an LLM; agentic-off must be fully deterministic.
- **Cost blowout from eager agentic steps.** → Classifier-gated escalation;
  per-run budget cap; agentic recovery off on `main`.
- **Agent makes things worse.** → git snapshot + rollback on every step; "don't
  weaken/delete tests" rule; changes reviewed as a diff, never silent on `main`.
- **Eino API drift.** → Pin versions; isolate Eino calls behind `model` and
  `agent` packages so an upgrade touches few files.

---

## 8. Open questions (decide before M5; some before M2)

1. **Deploy target for v1:** DECIDED — both Docker Compose and Kubernetes, one
   per run, selected by `DEPLOYER_TARGET` (`compose` | `k8s`). k8s via `helm`
   (for `helm rollback`). The orchestrator executes the chosen target; it does
   not infer it.
2. **Fixer backend in CI:** Ollama Pro cloud (consistent with home) vs. a model
   the org provides. Affects the model-factory config used in Actions.
3. **Where the conveyor runs:** GH-hosted runner vs. self-hosted (self-hosted
   needed if the fixer must reach a private/local model endpoint).
4. **Registry & image naming convention** (ghcr.io vs. other; tag = sha?).
5. **Audit retention:** are `.agents/*` rationale files committed per run, kept
   as CI artifacts, or both?

---

## 9. Hand-off to the interactive layer

This document seeds `.agents/plan.md`. Suggested first driver task (Claude Code):
*"Read docs/depl-orch-requirements.md. Finish M1: make the deterministic pipeline
(build → test → docker-build → docker-push) green end-to-end against a sample Go
service with a hand-written Dockerfile. No LLM in the pipeline path yet. Write the
plan to .agents/plan.md first; delegate review to the OpenCode reviewer
(minimax-m3) before proposing a commit."*

Definition of done for M1: a sample repo runs through all four deterministic
stages to a pushed image, with no agentic code on the pipeline path, reviewed
independently, tests green.
