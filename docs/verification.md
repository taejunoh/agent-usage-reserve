# Verification

## 0.2.0: agent-usage-reserve (Codex and Claude Code)

Date: 2026-10-29. Version: 0.2.0. Local platform: macOS arm64. Local build toolchain: Go 1.27.3 (`go.mod` stays at Go 1.23); Go dependencies: standard library only. This entry covers local implementation and verification only. The repository rename, push, tag, release and tap publication have not been done.

### Passed locally

- `gofmt -l .` clean; `go vet ./...` and `GOOS=windows go vet ./...` clean; `go test -count=1 -race ./...` passed (per-task runs also covered `-count=3 -shuffle=on`; see `.superpowers/sdd/task-1-report.md` and `task-2-report.md`).
- Engine and CLI tests, retargeted from 0.1.0: byte-exact preservation (CRLF, missing final newline, recorded padding), private backups, permission preservation, symlink refusal, external-edit and lock handling, malformed and duplicate markers.
- New tests: full Claude lifecycle; agent detection (none, one, both); selection and prompt input (empty, 1-3, invalid then valid, three invalid aborts); non-interactive refusal without mutation; `--only`; cross-agent markers refused; legacy `codex-usage-reserve` marker status, migration on `enable`/`set`, removal on `disable`, and legacy plus new block refused.
- Codex policy text compared against the 0.1.0 output: identical apart from the marker lines. Claude policy text compared against the design spec.
- Cross-binary migration: a policy enabled by the 0.1.0 binary was reported by the new binary, rewritten to the new marker on `enable`/`set`, and removed by `disable` with the original bytes restored; the legacy backup was left untouched (task 1 report).
- Shell installer integration (`sh scripts/test-install.sh`), re-run for this entry: old-executable removal and report, repeat install, no-agent and refused-home exit paths, bad checksum leaving everything unchanged, version mismatch. `sh -n install.sh` clean.
- `scripts/build-release.sh 0.2.0` produced six `agent-usage-reserve_0.2.0_*` archives (macOS, Linux, Windows; amd64, arm64) and `checksums.txt`.
- `scripts/generate-formula.py 0.2.0 ... --renames ...` generated the formula and `formula_renames.json` from that manifest; `ruby -c` accepted the formula.
- Demo regenerated from the real binary in a temporary home with `.codex` and `.claude` (`enable --only all`, `status`, `disable`); both instruction files were restored byte-for-byte, and the transcript contains no real home paths.
- Every command in the READMEs was checked against `agent-usage-reserve --help`; `--only`, `set`, `status` and the non-interactive guidance were run against temporary homes.

### Not run locally

- `install.ps1` and `scripts/test-install.ps1` were not executed (no PowerShell here); the Windows job in CI runs them. Windows runtime behavior is not claimed as locally tested.
- Not exercised: the Homebrew install and upgrade path, the public installers, the repository rename and its redirects, the release workflow, and the Claude desktop app's Code tab with a live usage tool. Reserve behavior depends on that tool being exposed to the agent.

All mutation tests used disposable homes. The user's actual `~/.codex` and `~/.claude` were not modified.

### Publication order (pending)

1. Rename the GitHub repository to `taejunoh/agent-usage-reserve`. Done 2026-10-29; the old web, raw installer, API, release asset and git URLs redirect, and the 0.1.0 one-line installer still installs and enables through the old URL.
2. Push the branch and the tag `v0.2.0` without updating `main`. The release workflow marks a release Latest only when its commit is already on `main`, so `v0.2.0` publishes without Latest and `releases/latest` stays `v0.1.0` for the old installers.
3. Let the release workflow publish, then verify the six archives and `checksums.txt`. Done 2026-10-29: `v0.2.0` published without Latest (`releases/latest` stays `v0.1.0`); all six SHA-256 sums match; each archive holds `agent-usage-reserve` (`.exe` on Windows) and `LICENSE` for the named OS and architecture with version `0.2.0`; the branch `install.sh` with `AGENT_USAGE_RESERVE_VERSION=0.2.0` replaced a sandboxed 0.1.0 install, removed the old executable, warned about another on PATH, and migrated the legacy Codex block at 10%.
4. In the tap, add the `agent-usage-reserve` formula and `formula_renames.json` and remove the old formula, then try `brew upgrade` against it.
5. Only then fast-forward `main` and immediately run `gh release edit v0.2.0 --latest`: the one-line installers read `main` and `releases/latest`, so the two change together.

Before announcing, check by hand that the Windows prompt appears under `irm | iex`.

## 0.1.0: initial release (codex-usage-reserve)

Date: 2026-10-28. Version: 0.1.0. Local platform: macOS arm64. Local build toolchain: Go 1.27.3; published releases and CI use Go 1.23; Go dependencies: standard library only.

### Passed locally

- `go test -count=1 -race ./...` and `go vet ./...`.
- CLI lifecycle in an isolated Codex home: enable at 10%, set to 15%, repeated enable retains 15%, status, disable. Original instruction bytes and SHA-256 were restored, including CRLF and no final newline. A second cycle preserved instructions appended by the user after installation.
- Core regression tests for backups, file modes, malformed and duplicate markers, symlinks, override precedence, lock ownership, external edits, file replacement, home precedence and version/help handling.
- Shell installer integration: default/latest and explicit versions, pinned asset downloads, successful enable, bad checksum preserving the previous executable and never enabling.
- Shell syntax checks for installer, build and integration scripts; GitHub workflow YAML parsing.
- Six release cross-builds: macOS/Linux/Windows, amd64/arm64. Independently verified each archive's SHA-256 and actual executable machine type, and exact archive members (executable and MIT LICENSE). macOS metadata sidecars are excluded.
- Homebrew formula generation from the actual final checksum manifest and Ruby syntax validation.
- Independent specification and bounded code-quality reviews. Material Windows snapshot-identity and release repository-selection findings were corrected and reviewed again.

### Passed after public deployment

- [Release-commit CI](https://github.com/taejunoh/codex-usage-reserve/actions/runs/40729512904) passed Go tests/vet on macOS, Linux and Windows, Unix installer integration on macOS/Linux, and the real Windows installer lifecycle using local release fixtures.
- The Windows installer test covers one latest-tag lookup, pinned downloads, explicit version normalization, actual executable installation and enablement, preservation of a custom 15% threshold on repeat installation, and checksum failure leaving the binary and instructions intact. A script-scope issue in the new test mock was fixed before release; the actual installer completed successfully in the original run.
- [Release workflow](https://github.com/taejunoh/codex-usage-reserve/actions/runs/40729512936) built and published all six binary archives and `checksums.txt`. Downloaded published assets were independently checked against the manifest and exact archive contents.
- Downloaded the public raw installer anonymously and installed the public latest release into isolated directories with spaces in their paths. Verified version 0.1.0, repeat installation preserving 15%, and exact instruction-byte recovery after disable.
- `brew install taejunoh/tap/codex-usage-reserve` succeeded against the published formula and public release asset. `codex-usage-reserve --version` returned `0.1.0`.
- `brew test --verbose taejunoh/tap/codex-usage-reserve` passed enable/status/disable in Homebrew's isolated test home.

### Execution boundaries

The anonymous public-download and Homebrew end-to-end checks ran on macOS arm64. Windows installer behavior was exercised on GitHub's Windows runner with network fixtures; public Windows HTTP download behavior was not separately exercised on a physical Windows computer. Linux runtime tests ran on GitHub's Linux runner. The other release architectures were cross-built.

The executable manages an instruction policy; it does not query usage directly, enforce an account cap, promise exact reserve percentages, or guarantee scheduled-task completion. The account usage-limits tool must be available to the Codex chat following the policy. Other editors can still race in the final snapshot-check/rename gap; detection is best effort.

All mutation tests used disposable homes. The user's actual global instructions were not modified by this project implementation.

### Published locations

- Public source: https://github.com/taejunoh/codex-usage-reserve
- Version 0.1.0: https://github.com/taejunoh/codex-usage-reserve/releases/tag/v0.1.0
- Homebrew formula: https://github.com/taejunoh/homebrew-tap/blob/main/Formula/codex-usage-reserve.rb

The user explicitly approved public deployment. The existing Homebrew tap was extended without changing its existing casks. Local output archives match the actual published release artifacts.
