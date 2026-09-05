# Synctest Scout

[English](README.md) | [한국어](README.ko.md)

[![CI](https://github.com/jisung-02/Synctest-Scout/actions/workflows/ci.yml/badge.svg)](https://github.com/jisung-02/Synctest-Scout/actions/workflows/ci.yml)
[![Coverage snapshot](docs/coverage.svg)](docs/coverage.md)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Find Go tests that wait for real time and migrate reviewed candidates to
`testing/synctest`, using familiar Go package patterns and command conventions.

```sh
synctest-scout scan ./...
synctest-scout fix -diff -run '^TestWithMaxDuration$' ./...

# Apply after reviewing the diff and the code it calls.
synctest-scout fix -run '^TestWithMaxDuration$' ./...
```

`fix` **writes source files**. Start with `fix -diff` to inspect proposed
changes. Like `go fix -diff`, it exits with status **1 when changes exist**
and 0 when there are no changes. A nonempty diff is not printed as an error.

## Install

Requires Go 1.25+ and Git. The CLI has no third-party Go library dependencies.

```sh
go install github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest
synctest-scout version
```

Or register it as a tool dependency in another Go module:

```sh
go get -tool github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest
go tool synctest-scout scan ./...
```

This repository already has its own `tool` directive:

```sh
git clone https://github.com/jisung-02/Synctest-Scout.git
cd Synctest-Scout
go tool synctest-scout help
go tool synctest-scout -C /path/to/project scan ./...
```

Version tags trigger archives for Linux, macOS, and Windows on amd64 and
arm64. See [releasing](docs/releasing.md) and [GitHub Releases](https://github.com/jisung-02/Synctest-Scout/releases)
for distribution details. Source installation also works before the first tagged release.

## Commands and options

| Command / option | Behavior |
|---|---|
| `help`, `help scan`, `help fix` | General help or command-specific flags |
| `version` | Version, commit, and build metadata |
| `-C dir` | Use a different working directory; place before the command |
| `scan [packages]` | List timing-related tests and review notes |
| `scan -json [packages]` | Emit a structured JSON report |
| `fix [packages]` | Rewrite candidates and print changed file names |
| `fix -diff [packages]` | Print unified diffs without writing |
| `fix -n [packages]` | Print files that would change without writing |
| `fix -run regexp [packages]` | Select full test paths using a regular expression |
| `scan/fix -tags a,b` | Select files using Go build tags |

The default package is `.`. Use `./...`, `./internal/...`, multiple packages,
or explicit `*_test.go` files. Flags must precede package arguments.
`-run` matches the full slash-separated path, such as
`TestDo/context_canceled`; it is not identical to `go test`'s hierarchical
subtest matching implementation.

Package discovery uses `go list -mod=readonly`, honoring the current platform,
build tags, and nested module boundaries. Selected tests are not executed,
but the Go command may access module proxies and caches. Discovery does not
modify `go.mod` or `go.sum`.

## Migration scope

- Detects sleeps, timers, tickers, context deadlines/timeouts, and selected
  testify Eventually calls.
- Recognizes import aliases and package names shadowed by local variables.
- Selects top-level tests and leaf subtests with literal names.
- Preserves a leading `t.Parallel()` outside the synctest bubble.
- Adds `synctest.Wait()` after standalone `time.Sleep` statements.
- Leaves old-Go compatibility, visible external I/O, and existing synctest
  structures for manual review.
- Prepares all rewrites before applying them and checks that the source has
  not changed. Replaces each file using a temporary file and rename while
  preserving permissions. This is not a multi-file transaction.
- Refuses to modify files outside the selected root. Use `-C` to set the project root.

`review` means no directly visible blocker was found. `manual` means the
candidate is excluded from automatic rewriting. **Neither classification is
proof of semantic safety.** Review transitive calls, shared state, external
I/O, goroutine lifetimes, and assertions that intentionally measure real
elapsed time. Adding `Wait` can also change synchronization order.

Timing calls hidden inside helpers and dynamically named table-driven
subtests can be missed. Further migration in a file that already imports
`testing/synctest` requires manual integration. Not every concurrency test
can be migrated automatically.

The prototype's `patch -file FILE -test NAME -reviewed` remains available for
compatibility. Prefer `fix -diff -run ...` for new workflows.

## CI and coverage

[Coverage report](docs/coverage.md) · [CI runs](https://github.com/jisung-02/Synctest-Scout/actions/workflows/ci.yml) ·
[Testing guide](docs/testing.md)

- Go 1.25 / 1.26 / 1.27 × Linux / macOS / Windows: nine matrix entries.
- All tests with `-race -shuffle=on -count=3`.
- Selected core regressions repeated 50 times with CPU settings 1, 2, and 4.
- Source-input and patch-round-trip fuzzing, 60 seconds each.
- Integration tests that apply real diffs, compile the result, and run race tests.
- A **95% overall / 90% per-package statement coverage** gate.
- gofmt, go vet, Actions syntax checks, and report/release script tests.
- Six-platform archive generation, checksum verification, and a host binary smoke test.

The `coverage-report` CI artifact contains line-level HTML, the raw profile,
function coverage, JSON and Markdown summaries, and an SVG badge. Each CI run
also writes a Job Summary. The README coverage badge is a **committed
snapshot**, not a live CI coverage measurement.

```sh
make lint
make test
make coverage   # Refresh the HTML report and README coverage snapshot.
make stress
make fuzz
```

On Windows, use commands such as `python scripts/quality.py test` directly.
Python 3.12+ is required; no additional Python packages are needed.

## Public-repository experiment

[Report](research/REPORT.md) · [Raw measurements](research/results.json) ·
[Generated patches](research/patches)

The initial prototype examined three public repositories and migrated three
`sethvargo/go-retry` tests. A batch of 20 runs of its 250ms wait test fell
from a median **5.05s to 0.013s**. This does not represent a whole-project CI speedup.

```sh
# Downloads and executes third-party tests in temporary checkouts.
python3 research/experiment.py
```

Normal CI tests the tool itself using local fixtures, rather than depending
on the network or changes in external repositories.

## Contributing and support

Read [CONTRIBUTING.md](CONTRIBUTING.md) to get started. For help, see
[SUPPORT.md](SUPPORT.md). Community participation follows the
[Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities according to
[SECURITY.md](SECURITY.md).

English is the primary documentation language. Korean translations use the
same filename with a `.ko.md` suffix and link back to the English version.
Keep both versions aligned when changing documented behavior.

## License

Original project code is available under the [MIT License](LICENSE).
Research patches preserve their upstream Apache-2.0 terms; see
[third-party notices](THIRD_PARTY_NOTICES.md).

Official behavior reference: [testing/synctest](https://pkg.go.dev/testing/synctest).
