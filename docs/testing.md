# Testing and coverage

[English](testing.md) | [한국어](testing.ko.md)

The required GitHub Actions check is **Quality gate**. It depends on every
matrix test, lint, coverage, fuzz, stress, and release-packaging job. A failure
or skipped dependency fails the gate. The release workflow calls this same
CI workflow at the tag commit before it builds or publishes release assets.

## Checks

| Check | Scope |
|---|---|
| Compatibility | Go 1.25.x, 1.26.x, 1.27.x on Ubuntu, macOS, Windows |
| Regression | All Go tests with `-race -shuffle=on -count=3` |
| Integration | Generated patch → Git apply → real Go compilation and race tests; built CLI subprocess |
| Package behavior | `go list` patterns, duplicate inputs, build tags, nested modules, files, and `-C` |
| Filesystem behavior | Preview is read-only; permissions survive writes; stale files and symlinks are rejected |
| Stress | Selected core regressions, `-race -count=50 -cpu=1,2,4 -shuffle=on` |
| Fuzz | Arbitrary source and generated-patch round trips, 60 seconds each |
| Static checks | gofmt, go vet, pinned actionlint |
| Supporting scripts | Weighted coverage, malformed profiles, independent package gate, release tags, deterministic archives |
| Release smoke | Six archives, manifest and SHA-256 validation, host binary version/help execution |

The stress suite intentionally focuses on core regressions instead of
rebuilding the integration fixture 150 times. Full integration runs in every
OS/Go matrix entry. The two fuzz targets never execute arbitrary fuzzed Go
source. A fuzz failure's reproducer is uploaded as an artifact.

## Coverage

`python scripts/quality.py coverage` runs:

```sh
go test -race -covermode=atomic -coverpkg=./... -coverprofile=coverage/coverage.out -shuffle=on -count=1 -timeout=5m ./...
```

The gate requires **95% overall statement coverage and 90% in every Go
package**. No Go package or generated production source is excluded. Block
coverage is deduplicated and weighted by statement count, not averaged across
packages. Empty, malformed, or inconsistent profiles fail closed.

Each CI run uploads `coverage-report` containing:

- `index.html`: Go's line-by-line source coverage viewer; download the artifact
  and open this file in a browser.
- `coverage.out`: original Go coverage profile.
- `functions.txt`: `go tool cover -func` output.
- `summary.md`, `summary.ko.md`, `summary.json`: aggregate and package totals, thresholds,
  source fingerprint and toolchain version.
- `coverage.svg`: report badge.

The markdown report also appears in the Actions Job Summary. The README
badge links to a committed snapshot in `docs/coverage.md`. Refresh that
snapshot with `make coverage` or
`python scripts/quality.py coverage --snapshot`. It is intentionally labeled
as a snapshot; an unconfigured remote is never presented as passing CI.

Statement coverage does not measure branch coverage, establish mutation
coverage, or prove that all suggested rewrites preserve semantics. The CLI
entry point is tested as a subprocess, but the parent test profile may not
count its one-line `os.Exit` statement. Some exceptional filesystem failures
also remain uncovered.

## Reproduce locally

```sh
python3 scripts/quality.py lint
python3 scripts/quality.py test
python3 scripts/quality.py coverage --snapshot
python3 scripts/quality.py stress
python3 scripts/quality.py fuzz --fuzztime=60s
```

Requires Go 1.25+, Git, and Python 3.12+. The Go/Python test suites have no
third-party library dependencies. Workflow linting additionally uses the
pinned actionlint command shown in `.github/workflows/ci.yml`.

Local validation during setup: Go 1.25.0 and Go 1.27.1 on macOS ARM64 passed
race tests. Both fuzz targets passed 30-second local campaigns. Six release
archives were built and the macOS ARM64 artifact was executed. The other OS
and version combinations are configured for GitHub-hosted CI and are not
claimed as already executed there.
