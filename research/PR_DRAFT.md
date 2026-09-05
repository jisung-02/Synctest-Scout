# Draft only — not submitted

[English](PR_DRAFT.md) | [한국어](PR_DRAFT.ko.md)

Target: sethvargo/go-retry at f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502.

Suggested title: Use synctest for backoff expiry and context timeout tests

The backoff expiry test currently waits 250ms per run, and two context tests
wait for 50ms and 10ms deadlines. Wrap their bodies in `synctest.Test` so the
timers advance in virtual time, preserving the leading `t.Parallel` calls
outside each bubble. Add `synctest.Wait` after the explicit sleeps in the
backoff expiry test. The module already declares Go 1.25.

On one macOS ARM64 machine running Go 1.27.1, the median time for a batch of
20 backoff expiry runs decreased from 5.050s to 0.0126s, excluding compilation
and including process startup. The full suite passed with `-race -count=10`
both before and after. Each selected test passed 60 times in each version.
Changing the production expiry condition from `<= 0` to `< 0` caused the
converted backoff test to fail at the exact expiry boundary.

Review consideration: `deadline_exceeded` now measures elapsed virtual time.
It verifies the deadline/backoff relationship, but does not measure real
scheduler latency. Omit or split that change if the real-time assertion is
an intentional performance contract.

Attached local patches:

- [Backoff expiry](patches/go-retry-backoff.patch)
- [Context timeouts](patches/go-retry-context.patch)

Full methodology and limitations: [experiment report](REPORT.md).
