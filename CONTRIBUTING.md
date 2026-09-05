# Contributing to Synctest Scout

[English](CONTRIBUTING.md) | [한국어](CONTRIBUTING.ko.md)

Contributions are welcome: bug reports, reproducible migration cases,
documentation improvements, and focused code changes.

## Before starting

- Search existing [issues](https://github.com/jisung-02/Synctest-Scout/issues)
  and pull requests for related work.
- Open an issue before a substantial API, CLI, or analysis change so the
  intended behavior can be discussed before implementation.
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md).
- Report vulnerabilities through the [security policy](SECURITY.md), rather
  than posting exploit details in a public issue.

## Development setup

Install Go 1.25 or later, Git, and Python 3.12 or later. No additional Go or
Python library dependencies are needed for the project test suites.

```sh
git clone https://github.com/jisung-02/Synctest-Scout.git
cd Synctest-Scout
go tool synctest-scout help
make build
```

On Windows, run the Python commands directly instead of using Make.

## Checks before submitting

```sh
python3 scripts/quality.py lint
python3 scripts/quality.py test
python3 scripts/quality.py coverage --snapshot
```

For parser or rewrite changes, also run:

```sh
python3 scripts/quality.py stress
python3 scripts/quality.py fuzz --fuzztime=60s
```

CI requires 95% overall statement coverage and 90% in each Go package. Do not
exclude code or weaken assertions merely to satisfy the threshold. Add tests
that distinguish correct behavior from a plausible regression. See
[testing documentation](docs/testing.md) for the complete CI matrix and
coverage artifacts.

The public-repository experiment is optional and separate from ordinary CI:

```sh
python3 research/experiment.py
```

It downloads and executes third-party Go tests in temporary checkouts.

## Code and behavior guidelines

- Keep the Go 1.25 compatibility floor unless a version change is explicitly
  part of the proposal.
- Run `gofmt` on Go changes. Keep user-facing output and primary documentation in English. Maintain matching
  `.ko.md` Korean translations with language links, and update both versions
  when documented behavior changes.
- Preserve the familiar Go command behavior, including package patterns,
  `-C`, build tags, and the exit status of `fix -diff`.
- Treat source preservation, scope boundaries, and diagnostics as correctness
  requirements. A preview must never write project files.
- Test import aliases, shadowing, subtests, comments, and repeat invocation
  when changing the rewriter.
- Distinguish a syntactic candidate from a proven safe transformation. Do not
  silently broaden the advertised safety guarantees.
- Keep ordinary tests self-contained. Do not add external network calls or
  upstream repository downloads to the normal test suite.

## Pull requests

Keep each pull request focused. Explain the concrete problem, the resulting
behavior, and the checks you ran. Include a minimal reproducer or regression
test for a bug fix. Update the README, command help, and changelog when the
user-facing behavior changes. Refresh the coverage snapshot for Go changes.

By submitting a contribution, you agree that your original contribution is
licensed under the project's [MIT License](LICENSE). Identify any third-party
material and preserve its license and attribution.
