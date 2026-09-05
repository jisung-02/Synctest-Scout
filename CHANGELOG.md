# Changelog

[English](CHANGELOG.md) | [한국어](CHANGELOG.ko.md)

Notable user-facing changes are recorded here. There are no tagged releases yet.

## Unreleased

### Added

- Timing-related Go test discovery with JSON and text reports.
- `fix`, `fix -diff`, `fix -n`, regular-expression selection, build tags,
  package patterns, and `-C` support.
- `go tool synctest-scout` integration and version metadata.
- Source-preserving rewrites, import alias handling, subtest selection,
  parallel-test preservation, and stale-source checks.
- A compatibility `patch` command for the initial prototype.
- Cross-platform CI, race and stress tests, fuzz targets, coverage gates,
  HTML reports, and a README coverage snapshot.
- Tag-triggered CLI archives for six OS/architecture combinations, with
  checksums and release smoke tests.
- Reproducible research artifacts from three public Go repositories.
