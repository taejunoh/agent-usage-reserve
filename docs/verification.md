# Initial release verification

Date: 2026-10-28. Version: 0.1.0. Local platform: macOS arm64. Local build toolchain: Go 1.27.3; published releases and CI use Go 1.23; Go dependencies: standard library only.

## Passed locally

- `go test -count=1 -race ./...` and `go vet ./...`.
- CLI lifecycle in an isolated Codex home: enable at 10%, set to 15%, repeated enable retains 15%, status, disable. Original instruction bytes and SHA-256 were restored, including CRLF and no final newline. A second cycle preserved instructions appended by the user after installation.
- Core regression tests for backups, file modes, malformed and duplicate markers, symlinks, override precedence, lock ownership, external edits, file replacement, home precedence and version/help handling.
- Shell installer integration: default/latest and explicit versions, pinned asset downloads, successful enable, bad checksum preserving the previous executable and never enabling.
- Shell syntax checks for installer, build and integration scripts; GitHub workflow YAML parsing.
- Six release cross-builds: macOS/Linux/Windows, amd64/arm64. Independently verified each archive's SHA-256 and actual executable machine type, and exact archive members (executable and MIT LICENSE). macOS metadata sidecars are excluded.
- Homebrew formula generation from the actual final checksum manifest and Ruby syntax validation.
- Independent specification and bounded code-quality reviews. Material Windows snapshot-identity and release repository-selection findings were corrected and reviewed again.

## Passed after public deployment

- [Release-commit CI](https://github.com/taejunoh/codex-usage-reserve/actions/runs/40729512904) passed Go tests/vet on macOS, Linux and Windows, Unix installer integration on macOS/Linux, and the real Windows installer lifecycle using local release fixtures.
- The Windows installer test covers one latest-tag lookup, pinned downloads, explicit version normalization, actual executable installation and enablement, preservation of a custom 15% threshold on repeat installation, and checksum failure leaving the binary and instructions intact. A script-scope issue in the new test mock was fixed before release; the actual installer completed successfully in the original run.
- [Release workflow](https://github.com/taejunoh/codex-usage-reserve/actions/runs/40729512936) built and published all six binary archives and `checksums.txt`. Downloaded published assets were independently checked against the manifest and exact archive contents.
- Downloaded the public raw installer anonymously and installed the public latest release into isolated directories with spaces in their paths. Verified version 0.1.0, repeat installation preserving 15%, and exact instruction-byte recovery after disable.
- `brew install taejunoh/tap/codex-usage-reserve` succeeded against the published formula and public release asset. `codex-usage-reserve --version` returned `0.1.0`.
- `brew test --verbose taejunoh/tap/codex-usage-reserve` passed enable/status/disable in Homebrew's isolated test home.

## Execution boundaries

The anonymous public-download and Homebrew end-to-end checks ran on macOS arm64. Windows installer behavior was exercised on GitHub's Windows runner with network fixtures; public Windows HTTP download behavior was not separately exercised on a physical Windows computer. Linux runtime tests ran on GitHub's Linux runner. The other release architectures were cross-built.

The executable manages an instruction policy; it does not query usage directly, enforce an account cap, promise exact reserve percentages, or guarantee scheduled-task completion. The account usage-limits tool must be available to the Codex chat following the policy. Other editors can still race in the final snapshot-check/rename gap; detection is best effort.

All mutation tests used disposable homes. The user's actual global instructions were not modified by this project implementation.

## Published locations

- Public source: https://github.com/taejunoh/codex-usage-reserve
- Version 0.1.0: https://github.com/taejunoh/codex-usage-reserve/releases/tag/v0.1.0
- Homebrew formula: https://github.com/taejunoh/homebrew-tap/blob/main/Formula/codex-usage-reserve.rb

The user explicitly approved public deployment. The existing Homebrew tap was extended without changing its existing casks. Local output archives match the actual published release artifacts.
