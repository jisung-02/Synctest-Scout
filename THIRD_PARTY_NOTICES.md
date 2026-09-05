# Third-party notices

[English](THIRD_PARTY_NOTICES.md) | [한국어](THIRD_PARTY_NOTICES.ko.md)

Original Synctest Scout code is licensed under the [MIT License](LICENSE).
The CLI has no third-party Go library dependencies.

## Research patches derived from go-retry

The files in `research/patches/` include original source and modified test
code from [sethvargo/go-retry](https://github.com/sethvargo/go-retry), pinned
to commit `f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502`.

That source is licensed under Apache-2.0. A copy of the original license is
preserved at [research/licenses/go-retry-APACHE-2.0.txt](research/licenses/go-retry-APACHE-2.0.txt).
The patches add `testing/synctest` wrappers and synchronization calls to the
selected upstream tests; they are not unmodified upstream files.

The experiment also reads pinned checkouts of cenkalti/backoff and
avast/retry-go. Those checkouts are cached outside the tracked source tree
and are not distributed in this repository or the CLI release archives.
