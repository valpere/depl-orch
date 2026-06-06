# depl-orch

A deterministic deployment conveyor in Go with **bounded** agentic recovery.

The pipeline — **build → test → docker-build → docker-push → deploy** — is plain,
reproducible Go. LLM "thinking" is confined to individual recovery/generation
steps (fix a failing test, generate a Dockerfile), never to control flow. See
[`docs/depl-orch-requirements.md`](docs/depl-orch-requirements.md) for the full
design and roadmap, and [`AGENTS.md`](AGENTS.md) for agent/contributor rules.

## Status

| Milestone | Scope | State |
|-----------|-------|-------|
| M0 | Scaffold (module, layout) | ✅ done |
| M1 | Deterministic pipeline (build → test → docker-build → docker-push), no LLM | ✅ done |
| **M2** | **Backend-agnostic model factory + fix-test recovery with git rollback** | ✅ done |
| M3 | Cost-aware classifier | ⏳ next |
| M4 | More agentic steps (fix-build, generate-dockerfile, fix-workflow) | — |
| M5 | Deploy stage (compose \| k8s) + GitHub Actions + branch-gating | — |
| M6 | Observability (Eino callbacks → Prometheus → Grafana) | — |

## Usage (M1)

The `deployer` binary runs the deterministic pipeline against one repository,
configured entirely from the environment:

```bash
go build -o bin/deployer ./cmd/deployer

DEPLOY_WORKDIR=./examples/sample-service \
DEPLOY_IMAGE=docker.io/pereval/depl-orch-sample:dev \
DEPLOY_PUSH=true \
  ./bin/deployer
```

| Env var | Default | Meaning |
|---------|---------|---------|
| `DEPLOY_WORKDIR` | `.` | repository to ship |
| `DEPLOY_DOCKERFILE` | `Dockerfile` | Dockerfile path (abs, or relative to workdir) |
| `DEPLOY_IMAGE` | *(required)* | full image reference, e.g. `docker.io/pereval/app:tag` |
| `DEPLOY_PUSH` | `true` | push the built image |

Registry auth stays **out-of-band** (an existing `docker login` locally, or a
login step in CI) — the orchestrator never reads credentials into code or logs.

Exit code: `0` success, `1` pipeline failure, `2` configuration error.

### Agentic recovery (M2, opt-in)

Off by default — the pipeline is deterministic unless `DEPLOY_RECOVER=true`. When
on, a failing **test** stage is handed to a bounded `fix-test` agent: it snapshots
the tree, lets a model edit source (never tests) through repo-scoped read/write
tools, re-runs `go test`, and **rolls back** if it can't fix it within the budget.
Control flow stays plain Go; the model only fills in individual tool calls.

| Env var | Default | Meaning |
|---------|---------|---------|
| `DEPLOY_RECOVER` | `false` | enable fix-test recovery |
| `DEPLOY_MAX_RETRIES` | `1` | recovery attempts per stage |
| `MODEL_BACKEND` | `ollama` | `ollama` \| `openai` \| `anthropic` |
| `MODEL_ID` | *(required if recovering)* | model id, e.g. `qwen3-coder-next:cloud` |
| `MODEL_BASE_URL` | Ollama local | endpoint |
| `MODEL_API_KEY` | — | key for openai / anthropic |

## Development

```bash
gofmt -w .            # format (CI rejects unformatted code)
go vet ./...          # lint
go build ./...        # build
go test ./...         # unit tests — hermetic, no docker required
```

End-to-end test (needs docker + a logged-in registry; **pushes** an image):

```bash
go test -tags integration ./test/e2e/...      # set E2E_IMAGE / E2E_PUSH to override
```

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs gofmt / vet /
build / test on every push to `main` and every PR, for both the root module and
`examples/sample-service`.
