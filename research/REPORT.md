# Public-repository migration experiment

[English](REPORT.md) | [한국어](REPORT.ko.md)

Run on 2026-09-05 using Go 1.27.1 on macOS ARM64.

The initial prototype demonstrated a practical benefit: it found six direct
timing candidates in 17 test files across three pinned public repositories,
then migrated and tested three cases from one repository. Maintainer adoption
and market demand have not been established.

## Candidate discovery

| Repository | Pinned commit | Test files | Direct candidates | Assessment |
|---|---|---:|---:|---|
| cenkalti/backoff | [ffcfd8ab39e2](https://github.com/cenkalti/backoff/tree/ffcfd8ab39e2910a1180ba0b7a02a52f0485adc9) | 6 | 1 | Declares Go 1.23; needs a compatibility decision |
| avast/retry-go | [5bccbfa9340df](https://github.com/avast/retry-go/tree/5bccbfa9340dfe6609f4ecfce30c971e2756c796) | 6 | 1 | Declares Go 1.20; needs a compatibility decision |
| sethvargo/go-retry | [f6b3e1a9f1c5](https://github.com/sethvargo/go-retry/tree/f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502) | 5 | 4 | Declares Go 1.25; selected the three cases below |

Raw discovery reports: [backoff](scans/backoff.json), [retry-go](scans/retry-go.json),
[go-retry](scans/go-retry.json).

Candidate counts are not an exhaustive inventory of migratable tests. The
prototype misses timers in helpers and dynamically named table-driven
subtests. Some backoff tests under `t.Run(tc.name, ...)` were therefore not
included. Conversely, `TestExponentialBackoff_ConcurrentOverflow` uses
`time.After` as a failure watchdog; its normal execution has little wait time
to remove, so it was not selected.

## Measurements

Each trial invoked a precompiled test binary with 20 repetitions of one
selected test. There were three trials per test, interleaving before and
after. The table reports the median batch wall time, excluding compilation
but including process startup. Timing runs did not enable the race detector.

| Selected test | Before, 20 runs | After, 20 runs | Batch wall-time ratio |
|---|---:|---:|---:|
| TestWithMaxDuration | 5.050s | 0.0126s | About 401× |
| TestDo/context_canceled | 1.039s | 0.0103s | About 101× |
| TestDo/deadline_exceeded | 0.227s | 0.0131s | About 17× |

These are results for tests whose real waits were removed, not speedups for
an entire project or CI pipeline. The first after-migration batch for
`TestWithMaxDuration` took 0.558s; the other two took 0.0064s and 0.0126s.
Startup costs and host conditions materially affect these small values. All
raw measurements are preserved; this is not a microbenchmark claim.

- [Measurements, commits, and environment](results.json)
- [Repeated-run logs](logs/)
- [Backoff migration patch](patches/go-retry-backoff.patch)
- [Context migration patch](patches/go-retry-context.patch)

## Validation

- Each selected test passed 60 times before and 60 times after migration.
- The complete upstream suite passed `go test -race -count=10 -timeout=60s ./...`
  both before and after.
- The harness verified that the source changed and every selected test gained
  a `synctest.Test` wrapper.
- Mutating `WithMaxDuration` from `diff <= 0` to `diff < 0` caused the migrated
  test to fail with `should stop`, detecting the incorrect exact-deadline
  behavior at 250ms. The mutation was applied only in a temporary copy and restored.

[Before race log](logs/before-race.txt), [after race log](logs/after-race.txt),
[mutation result](logs/mutation.txt).

Passing repeated tests does not prove semantic equivalence or a reduced
flakiness rate. No failures of the original tests were observed in these runs.

## Manual review of the selected cases

`TestWithMaxDuration` creates its backoff state locally and calls an
implementation using `time.Now` and `time.Since`. There is no external I/O on
that path. Its 200ms and 50ms waits inspect expiry of local state, making the
virtual-time purpose clear.

`TestDo/context_canceled` constructs a local context and a pure callback. The
`Do` → `DoValue` path exits through a timer/context select. HTTP examples exist
elsewhere in the same file but are not called by this test. Rejecting a test
solely because its file imports `net/http` would miss this candidate.

`TestDo/deadline_exceeded` follows the same path. However, its `time.Since`
assertion measures real elapsed time before migration and virtual elapsed time
afterward. The migrated test checks the logical relationship between deadline
and backoff; it no longer measures real scheduler latency. A maintainer should
assess that change against the intended test contract independently.

Leading `t.Parallel()` calls remain outside the bubbles. Only standalone
sleeps in the backoff test gained `synctest.Wait()`. The upstream `go.mod` was
unchanged. Official constraints: [testing/synctest](https://pkg.go.dev/testing/synctest).

## Product implications

A small CLI can demonstrate value, but the current `review` classification
is not an automatic safety guarantee. Priorities for further work are:

1. Use package/type information to trace helpers and local call paths.
2. Distinguish waits from assertions intended to measure real latency.
3. Support dynamically named table-driven subtests.
4. Obtain maintainer feedback on patches in additional repositories.

The initial audience is teams already on Go 1.25+. For libraries retaining
older Go support, raising the compatibility floor may cost more than the test
runtime savings. No upstream issue or pull request was submitted; a
[maintainer-facing draft](PR_DRAFT.md) is included locally.

## Reproduction and attribution

Run `python3 research/experiment.py` from the project root. It downloads
pinned commits and performs rewrites, tests, and measurements in temporary
copies with independent Git roots. Reports remain under `research/`; cached
upstream clones remain in the ignored `.research/repos/` directory.

These are historical measurements from the initial prototype. The current
CLI uses Go package resolution, so later discovery results can differ.

Upstream go-retry code included in the patches is Apache-2.0 licensed. Its
[original license](licenses/go-retry-APACHE-2.0.txt) is preserved.
