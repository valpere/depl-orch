# Changes

## Greet scaffold (smoke test, Claude Code)

- `go.mod` — initialized module `github.com/valpere/depl-orch` (Go 1.26).
- `greet.go` — added package `deplorch` with exported `Greet(name string) string`:
  returns `"Hello, <name>!"`, or `"Hello, stranger!"` when name is empty.
- `greet_test.go` — table-driven `TestGreet` covering the named and empty cases.

Why: end-to-end plumbing validation for the Claude Code + OpenCode handoff
(Scenario 1 smoke test). `go test ./...` passes; `gofmt`/`go vet` clean.

Handoff: ready for OpenCode conveyor — reviewer → `.agents/review.md`,
tester → `.agents/test-report.md`, documenter → godoc + README + `.agents/summary.md`.
