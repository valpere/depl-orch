# M1 independent review — `depl-orch`

> reviewer: minimax-m3:cloud (Ollama Pro, lane 3 — independent of Claude family)
> scope: `.agents/scratch/m1.patch` + sources under `internal/pipeline/`, `internal/config/`, `cmd/deployer/`, `examples/sample-service/`
> spec: `docs/depl-orch-requirements.md` §3.1, §6 M1, §9 (definition of done)

Note on delivery: the `write` tool is unavailable in this environment, so I could not write the canonical `.agents/review.md` (which currently contains stale session noise from a prior failed attempt — recommend the driver overwrite it with this body). No code was edited.

## Verdict: **approve with minor fixes recommended before PR**

The M1 spine is sound. The layering rule is honored *by construction*: `internal/pipeline` has zero non-stdlib imports, so no future M2 agent package can leak in without an explicit import — that is the right shape. The fake-Commander seam is well-chosen and the unit tests are tight. Findings below are minor, plus one small bug.

---

## 1. Layering rule (§5 of spec)

- `internal/pipeline/{pipeline,exec,stages,pipeline_test}.go` — **PASS.** No imports outside stdlib (`context`, `fmt`, `log/slog`, `os/exec`, `path/filepath`, `time`, `errors`, `strings`, `testing`).
- `internal/config/config.go` — **PASS** (stdlib only).
- `cmd/deployer/main.go:11-12` — **PASS.** Wires `config` + `pipeline` only; no `agent`/`model` import.
- M2 hook is clean: a `Recoverable` interface added to `pipeline` and implemented under `internal/agent/` (per spec §5) will not force this file to change. Good.

## 2. Bugs / correctness

### B1. `getenvBool` silently swallows invalid values → wrong default behavior  *(minor, `internal/config/config.go:46-55`)*
If `DEPLOY_PUSH=maybe`, the function returns the *default* (true) with no error. A typo flips a push on, not off. Recommend: return the parse error (or at minimum log via `slog`). Same risk in `test/e2e/e2e_test.go:39-41` — `_ = err` on `strconv.ParseBool` silently uses `true` even for `E2E_PUSH=garbage`. Add a `t.Fatalf` on parse error.

### B2. `getenv` treats `" "` (whitespace) as set  *(minor, `internal/config/config.go:39-44`)*
Empty-string check is the only sentinel. A space-padded env value passes through. Not a security issue here, but worth `strings.TrimSpace` on `ImageRef` and `WorkDir`, or reject whitespace-only explicitly — `DEPLOY_IMAGE` is *required* but `"  "` will sail through `os.Getenv != ""` and into `docker build -t "  "` / `docker push "  "`.

### B3. `cmd/deployer/main.go:30` — `context.Background()` ignores signals  *(minor, but matters for CI)*
No `signal.NotifyContext`, no flag handling, no `-version`, no `-help`. `Ctrl-C` mid-`docker build` orphans the build context. Spec §3.4 says "Runs as a single step in GitHub Actions" — SIGTERM from a cancelled Actions run should propagate. Fix: `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()`. Also: if `Push=true` and `ImageRef` lacks a tag, docker defaults to `:latest` (silent); consider validating the ref shape.

### B4. `docker-push` skip returns `nil` without logging  *(small UX, `internal/pipeline/stages.go:50-52`)*
Other stages log start/end; the skip path produces neither, so the JSON log shows no `docker-push` event at all when `Push=false`. Either emit an `info("stage skipped", ...)` or keep the `start` log and add `stage skipped` on the `Push=false` branch. Helps grep-based audit (spec §3.5).

### B5. `runCmd` error message embeds raw bytes  *(cosmetic, `internal/pipeline/stages.go:63`)*
`fmt.Errorf("%s %v: %w\n%s", bin, args, err, tail(out, 2000))` — the trailing `%s` on `tail(out, 2000)` (a `[]byte`) renders only printable bytes; non-printable runs become garbled/empty. Also: the error string includes the full command line including `st.ImageRef`. If a future user puts a registry credential in the image ref (they shouldn't, but…), it lands in `os.Stderr` JSON. For M1 this is fine because `ImageRef` is not a secret by spec, but document the rule on the `State.ImageRef` doc-comment. The bigger issue: docker-build args on stderr will be the *full* path of `WorkDir` + `dockerfile`; if `WorkDir` is set to something sensitive, it leaks. For now, `WorkDir` is repo-local per spec, so low risk — flag for M5 when `DEPLOYER_TARGET` arrives.

## 3. Security

### S1. Command-injection surface — **OK for M1, watch for M2**
All command invocations use `exec.CommandContext` with discrete `args` (no `bash -c`, no string concat). Confirmed in:
- `internal/pipeline/exec.go:19` — `exec.CommandContext(ctx, name, args...)` ✅
- `internal/pipeline/stages.go:24,31,42,53` — discrete `args` ✅

Even hostile `DEPLOY_IMAGE="evil;rm -rf /"` becomes a single argv element to `docker build -t`/`docker push`, which docker itself will reject at the registry. **However**: when M2 injects LLM-generated paths into agentic steps, the doc *must* require the same `exec.CommandContext(name, args...)` shape — no shell. Add a comment to `Commander` (`internal/pipeline/exec.go:8-13`) explicitly forbidding a shell-interpreting wrapper, since that interface is what agent stages will use.

### S2. Path-traversal in `dockerfile` — **OK with one gap**
- `internal/pipeline/stages.go:38-40` — `filepath.IsAbs` check is correct, `filepath.Join(st.WorkDir, dockerfile)` is safe (Join cleans, no `..` escape of `WorkDir` — actually it does, `filepath.Join("/work", "../../etc/passwd")` → `/etc/passwd`).
- **Gap:** nothing stops `DEPLOY_DOCKERFILE=../../etc/shadow`. docker will then read `/etc/shadow` into the build context (a daemon-side read, not an orchestrator read, but it still ships to a remote daemon). For M1 this is acceptable because the binary is operator-controlled. Document the trust model on `Config.Dockerfile` doc-comment, or in M5 add a `filepath.Clean` + `strings.HasPrefix(dockerfile, st.WorkDir+string(filepath.Separator))` check (allowing `/abs/path` for the absolute case).

### S3. Secret leakage — **OK for M1**
- `internal/pipeline/stages.go:63` error message does **not** include env or any process state, only the command line.
- `cmd/deployer/main.go:16` slog is JSON to stderr; no `DEPLOY_*` values are logged. `cfg.ImageRef` is logged only on the success line (`main.go:34`) — and per spec the ref is not a secret. ✅
- `.gitignore:28-29` keeps `.env` out of git. ✅
- The integration test `test/e2e/e2e_test.go:36` hard-codes `docker.io/pereval/depl-orch-sample:m1-test` — fine for local e2e, but the test will *push* to Docker Hub. Consider gating push behind an explicit `E2E_PUSH=true` default-off in CI to avoid accidental public pushes from forks.

### S4. `os/exec` PATH resolution — **OK but worth pinning**
`internal/pipeline/exec.go:19` uses bare `name` (PATH lookup) for `go` and `docker`. If `WorkDir` is attacker-controlled, it can shadow `go`/`docker` on PATH via `WorkDir/go`. Not exploitable in M1's threat model (operator runs the binary), but in CI/M5 the agentic layer may run with `WorkDir` influenced by repo contents. For now: leave it, but add a TODO referencing `exec.LookPath` + a future `Cmd.Path` resolver for M5.

## 4. Go idioms

- `internal/pipeline/pipeline.go:16-22` — `State` doc-comment claims "lives in fields and the git working tree", but no field is annotated as such. Fine for M1, consider grouping via embedding when M2 adds `SnapshotID`.
- `internal/pipeline/pipeline.go:51-67` — `Run` re-derives `log` on every call (cheap), but the `r.Log == nil` branch is dead in the only caller (`main.go:27` always sets `Log`). Either drop the nil-check (and document `Log` is required) or initialize `Runner{}` in a constructor. Minor.
- `internal/pipeline/pipeline.go:63` — `fmt.Errorf("stage %q: %w", ...)` is correct. ✅
- `internal/pipeline/stages.go:23-25, 30-32` — repeating `runCmd(...)` four times is fine, but consider a small `execStage{name, dir, bin, args}` value type to make `DefaultStages` table-driven. YAGNI for 4 stages.
- `internal/pipeline/stages.go:69-73` — `tail` allocates a new slice on every error. For 2 KB and a rare error path this is fine.
- `internal/config/config.go:39-55` — `getenv`/`getenvBool` are unexported helpers; a single `LookupEnv` + parse could collapse them. Cosmetic.
- `cmd/deployer/main.go:15-35` — clean, no globals, no init. ✅
- `examples/sample-service/main.go:25` — `cmp.Or(os.Getenv("PORT"), "8080")` is idiomatic 1.22+. ✅
- `examples/sample-service/main.go:27` — `log.Fatal(http.ListenAndServe(addr, nil))` — fine for an example. No graceful-shutdown needed here, but a real service shouldn't take this shortcut (out of scope for M1).

## 5. Test coverage

- `internal/pipeline/pipeline_test.go:31-95` — covers order, stop-on-fail, skip-on-Push=false, output capture, absolute Dockerfile. Solid for the public surface.
- **Missing test** *(recommended)*: a stage that fails *before* writing output (e.g. `exec` returns `(nil, error)`). Current fake always returns `[]byte("output of ...")`. `st.Outputs[stage]` is set even on failure — good — but the error-message formatting path (the `%w\n%s` in `stages.go:63`) is not exercised when `out` is empty or short. One more fake case (`out: nil`) is enough.
- **Missing test** *(recommended)*: a stage that takes >0 wall time asserting that `elapsed` is logged. The current fakes are instant, so the JSON `elapsed` field is "0s" — the code works, but the contract isn't asserted. Use `time.Sleep(10*time.Millisecond)` and `>= 10*time.Millisecond` assertion, or accept the field is best-effort.
- `test/e2e/e2e_test.go:24-48` — integration test is build-tagged correctly. ✅ One concern: it pushes by default; combined with S3, the test should require `E2E_PUSH=true` to opt in.

## 6. Spec alignment (§3.1, §9)

- §3.1 stages present in correct order: ✅
- §3.1 "each logs start/end/elapsed": ✅ (with the B4 caveat on the skip path)
- §3.4 "exit code reflects pipeline result": `main.go:21,32` — config error → 2, pipeline error → 1, success → 0. Conventional. ✅
- §9 "all four deterministic stages … reviewed independently": pending this review.
- M1 DoD gates `gofmt`/`go vet`/`go test ./...` — **not run by me** (no `bash`); the code is formatted by inspection and `gofmt`/vet should be clean. `pipeline_test.go:5` imports `errors` ✅, no unused imports spotted.

## 7. Required fixes (blockers for PR)

None. The spine works and the layering rule holds.

## 8. Recommended fixes (non-blocking, pick up before M2)

1. **B1** — surface `strconv` parse errors in `getenvBool` and `e2e_test.go`.
2. **B3** — `signal.NotifyContext` in `main.go`.
3. **B2** — `strings.TrimSpace` on `ImageRef` + reject whitespace-only.
4. **S1** — add anti-shell comment to `Commander` interface.
5. Add the two missing unit tests from §5.
6. **B5** — doc-comment the trust boundary on `State.WorkDir` / `ImageRef` for the M5 deployer work.

## 9. Cross-checks worth a second pair of eyes

- Confirm `go test ./...` runs cleanly with no docker (the unit tests must be hermetic). Patch claims they are; I traced every `Commander` call in tests and all go through `fakeCommander`. ✅
- Confirm `gofmt -d .` is empty (visual inspection suggests it is; cannot run).
- Confirm the `integration_test.go` file the plan mentions at line 51 was *not* added — patch shows `test/e2e/e2e_test.go` instead. That's the right choice (e2e is under its own package, not the root), but the plan's "Files" section is now stale. Update the plan before merge.

---

**Recommendation:** approve M1; address B1/B2/B3 in a follow-up commit before the M2 (model factory) PR opens. The most important property — `pipeline` imports nothing non-stdlib — is intact, so M2 can land without churn here.
