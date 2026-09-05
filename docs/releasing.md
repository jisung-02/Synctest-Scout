# CLI releases

[English](releasing.md) | [한국어](releasing.ko.md)

Synctest Scout is distributed as a CLI through GitHub Releases. The release
workflow runs only for a pushed `v*` tag and first runs the entire reusable CI
workflow at that same commit. Building/testing has read-only repository
permissions; only the final publish job has `contents: write`.

## Local packaging

```sh
python3 scripts/release.py --version v0.0.0-dev --output dist/snapshot
python3 scripts/smoke_release.py dist/snapshot
```

The output directory must be empty to prevent old artifacts being mixed into
a new release. Choose another directory for a second build.

For Linux, macOS, and Windows, each in amd64 and arm64, this generates:

- a tar.gz archive on Linux/macOS or a zip archive on Windows;
- English and Korean documentation, the MIT license, and third-party notices;
- a CLI built with `CGO_ENABLED=0`, `-trimpath`, version and commit metadata;
- `manifest.json` with platform, size, SHA-256 and toolchain metadata;
- `checksums.txt` covering the archives and manifest.

The build uses Go and Python's standard library directly. Packaging fixes
archive timestamps, modes and owner metadata. Identical binary inputs produce
identical archives; this is not a promise of identical binaries across Go
versions. Git is required by the CLI's diff mode, and Go is required for
package resolution.

The smoke script verifies every checksum and the complete six-platform
manifest, then extracts and runs only the artifact matching the current host.
Cross-compiled artifacts for other platforms are built but not executed by
this script.

## Activate on GitHub

The canonical repository is [jisung-02/Synctest-Scout](https://github.com/jisung-02/Synctest-Scout).
The module path is `github.com/jisung-02/Synctest-Scout`; source installation
uses `go install github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest`.
A separate server deployment is not required for this CLI.

1. Confirm that the **Quality gate** succeeds at the intended commit.
2. Configure that gate as a required check in repository rules if desired.
3. When the intended version is ready, create and push an annotated tag:

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

This triggers actual publication after checks pass. A tag such as
`v0.1.0-rc.1` becomes a GitHub prerelease. Invalid version strings fail before
packaging. The workflow uses the repository's built-in `GITHUB_TOKEN`; no
external publishing account or third-party coverage token is needed.
