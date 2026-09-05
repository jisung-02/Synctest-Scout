# Security policy

[English](SECURITY.md) | [한국어](SECURITY.ko.md)

## Supported code

This project is currently in initial development. Security fixes target the
latest `main` revision. No tagged release support window has been established
yet. Reports are reviewed on a best-effort basis; there is no guaranteed
response time or bug bounty program.

## Reporting a vulnerability

Use GitHub's [private vulnerability reporting form](https://github.com/jisung-02/Synctest-Scout/security/advisories/new)
when it is available. Do not disclose exploit details, credentials, or
sensitive source code in a public issue.

If the private reporting form is unavailable, open an issue titled
**Request for a private security contact**, containing no vulnerability
details. A maintainer can arrange an appropriate private channel.

A useful report includes:

- the affected commit or version, Go version, and operating system;
- the command and a minimal source example that trigger the issue;
- the expected behavior and actual impact;
- reproduction steps and any relevant boundary conditions;
- a proposed fix or mitigation, if known.

Coordinate disclosure with the maintainer while a report is being evaluated.

## Relevant security boundaries

Synctest Scout parses project files and invokes installed Go and Git tools.
Package discovery may access module proxies or the Go module cache. It does
not execute the selected project's tests during `scan` or `fix`.

`fix` modifies source files. `fix -diff` and `fix -n` are previews. The tool
checks file scope, rejects non-regular files, and detects changes to the
original source before applying a prepared rewrite. File replacement is
atomic per file, not a transaction across an entire repository.

The analyzer is syntax-based. It does not prove the absence of external I/O,
shared state, or changed synchronization behavior. Review proposed rewrites
and validate the resulting tests before relying on them.

The optional `research/experiment.py` script explicitly downloads and runs
third-party tests; it is not part of the normal CI test suite.
